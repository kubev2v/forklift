package hyperv

import (
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Firmware types
const (
	BIOS = "bios"
	UEFI = "uefi"
)

// Network types
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

type Builder struct {
	*plancontext.Context
}

// SMB credentials handled by CSI driver at pod mount time, not per-VM
func (r *Builder) Secret(_ ref.Ref, _, _ *core.Secret) error {
	return nil
}

// No per-VM config needed for HyperV
func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) error {
	return nil
}

func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	vm := &model.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return liberr.Wrap(err, "vm", vmRef.String())
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapFirmware(vm, object)
	r.mapInput(object)
	r.mapTpm(vm, object)
	r.mapNetworks(vm, object)
	r.mapCPU(vmRef, vm, object, usesInstanceType)
	r.mapMemory(vm, object, usesInstanceType)

	return nil
}

func (r *Builder) mapDisks(vm *model.VM, pvcs []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	for i, disk := range vm.Disks {
		pvc := r.findPVC(disk.ID, pvcs)
		if pvc == nil {
			r.Log.Info("PVC not found for disk, skipping",
				"diskID", disk.ID,
				"vmName", vm.Name)
			continue
		}
		volumeName := fmt.Sprintf("vol-%d", i)
		kVolumes = append(kVolumes, cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		})
		kDisks = append(kDisks, cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: cnv.DiskBusVirtio,
				},
			},
		})
	}
	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

func (r *Builder) findPVC(diskID string, pvcs []*core.PersistentVolumeClaim) *core.PersistentVolumeClaim {
	for _, pvc := range pvcs {
		if pvc.Annotations != nil {
			if pvc.Annotations[planbase.AnnDiskSource] == diskID {
				return pvc
			}
		}
	}
	return nil
}

func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	firmware := &cnv.Firmware{}
	if vm.Firmware == UEFI {
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &vm.SecureBoot,
			},
		}
		if vm.SecureBoot {
			object.Template.Spec.Domain.Features = &cnv.Features{
				SMM: &cnv.FeatureState{
					Enabled: &vm.SecureBoot,
				},
			}
		}
	} else {
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapTpm(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.TpmEnabled {
		// If the VM has vTPM enabled, we need to set Persistent in the VM spec.
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Persistent: ptr.To(true)}
	} else {
		// Force disable the vTPM
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Enabled: ptr.To(false)}
	}
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{
		{
			Type: "tablet",
			Name: "tablet",
			Bus:  cnv.InputBusVirtio,
		},
	}
}

// shouldUseQualifiedNetworkName determines whether to use namespace/nad-name format
// for Multus networks based on ForkliftController settings and namespace comparison.
func (r *Builder) shouldUseQualifiedNetworkName(nadNamespace, targetVMNamespace string) bool {
	// If global setting forces qualified names, always use qualified format
	if settings.Settings.Migration.MultusNetworkNameAlwaysQualified {
		return true
	}

	// If NAD and target VM are in different namespaces, use qualified format for safety
	if nadNamespace != targetVMNamespace {
		return true
	}

	// NAD and VM are in same namespace, use unqualified format
	return false
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	pool := planbase.NewNADPool()
	nicKeys, pairsBySource := r.buildNICResolver(vm.NICs)

	for i, nic := range vm.NICs {
		pair, allocated := planbase.AllocateNetwork(pool, pairsBySource[nicKeys[i]])
		var mapped *api.NetworkPair
		if allocated {
			mapped = &pair
		}

		// Skip if no valid mapping found or the destination type is Ignored
		if mapped == nil || mapped.Destination.Type == Ignored {
			continue
		}

		networkName := fmt.Sprintf("net-%d", numNetworks)
		numNetworks++

		kNetwork := cnv.Network{Name: networkName}
		kInterface := cnv.Interface{
			Name:       networkName,
			MacAddress: nic.MAC,
		}

		switch mapped.Destination.Type {
		case Multus:
			var networkName string
			if r.shouldUseQualifiedNetworkName(mapped.Destination.Namespace, r.Plan.Spec.TargetNamespace) {
				networkName = path.Join(mapped.Destination.Namespace, mapped.Destination.Name)
			} else {
				networkName = mapped.Destination.Name
			}
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: networkName,
			}
			kInterface.Bridge = &cnv.InterfaceBridge{}
		case Pod:
			fallthrough
		default:
			kNetwork.Pod = &cnv.PodNetwork{}
			kInterface.Masquerade = &cnv.InterfaceMasquerade{}
		}

		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces
}

func (r *Builder) buildNICResolver(nics []hyperv.NIC) ([]string, map[string][]api.NetworkPair) {
	networkCount := nicNetworkCount(nics)

	pairsBySource := map[string][]api.NetworkPair{}
	vlanQualifiedNetworks := map[string]bool{}
	if r.Map.Network != nil {
		for _, pair := range r.Map.Network.Spec.Map {
			network := &model.Network{}
			if err := r.Source.Inventory.Find(network, pair.Source.Ref); err != nil {
				continue
			}
			key := buildPairKey(network.ID, pair.Source.Vlan, networkCount)
			pairsBySource[key] = append(pairsBySource[key], pair)
			if pair.Source.Vlan != "" {
				vlanQualifiedNetworks[network.ID] = true
			}
		}
	}

	return buildNICKeys(nics, networkCount, vlanQualifiedNetworks), pairsBySource
}

// nicNetworkCount returns a map of network ID → number of NICs attached to it.
func nicNetworkCount(nics []hyperv.NIC) map[string]int {
	count := map[string]int{}
	for _, nic := range nics {
		count[nic.Network.ID]++
	}
	return count
}

// buildNICKeys produces a lookup key per NIC. VLAN suffixes are only appended
// when the NetworkMap has VLAN-qualified entries for that network. This ensures
// backward compatibility: if the map uses plain network IDs (no Vlan field),
// NIC keys remain plain too, preserving the old 1:1 fallback behavior.
func buildNICKeys(nics []hyperv.NIC, networkCount map[string]int, vlanQualifiedNetworks map[string]bool) []string {
	keys := make([]string, len(nics))
	for i, nic := range nics {
		if networkCount[nic.Network.ID] > 1 && nic.VlanId > 0 && vlanQualifiedNetworks[nic.Network.ID] {
			keys[i] = nic.Network.ID + "/" + strconv.Itoa(nic.VlanId)
		} else {
			keys[i] = nic.Network.ID
		}
	}
	return keys
}

// buildPairKey constructs the lookup key for a network map pair entry.
// The VLAN suffix is only added when the network has multiple NICs attached.
func buildPairKey(networkID, vlan string, networkCount map[string]int) string {
	if vlan != "" && networkCount[networkID] > 1 {
		return networkID + "/" + vlan
	}
	return networkID
}

func (r *Builder) mapCPU(vmRef ref.Ref, vm *model.VM, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: 1,
		Cores:   uint32(vm.CpuCount),
		Threads: 1,
	}
	if enableNestedVirt := r.NestedVirtualizationSetting(vmRef, false); enableNestedVirt != nil {
		policy := "optional"
		if !*enableNestedVirt {
			policy = "disable"
		}
		object.Template.Spec.Domain.CPU.Features = append(object.Template.Spec.Domain.CPU.Features,
			cnv.CPUFeature{Name: "vmx", Policy: policy},
			cnv.CPUFeature{Name: "svm", Policy: policy},
		)
	}
}

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
	memory := resource.NewQuantity(int64(vm.MemoryMB)*1024*1024, resource.BinarySI)
	object.Template.Spec.Domain.Resources.Requests = core.ResourceList{
		core.ResourceMemory: *memory,
	}
}

func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, disk := range vm.Disks {
		dv, dvErr := r.mapDataVolume(disk, dvTemplate)
		if dvErr != nil {
			err = dvErr
			return
		}
		dvs = append(dvs, *dv)
	}
	return
}

func (r *Builder) mapDataVolume(disk hyperv.Disk, dvTemplate *cdi.DataVolume) (*cdi.DataVolume, error) {
	storageClass := r.getStorageClass()
	dvSource := cdi.DataVolumeSource{
		Blank: &cdi.DataVolumeBlankImage{},
	}
	dvSpec := cdi.DataVolumeSpec{
		Source: &dvSource,
		Storage: &cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: *resource.NewQuantity(disk.Capacity, resource.BinarySI),
				},
			},
			StorageClassName: &storageClass,
		},
	}

	dv := dvTemplate.DeepCopy()
	dv.Spec = dvSpec
	if dv.ObjectMeta.Annotations == nil {
		dv.ObjectMeta.Annotations = make(map[string]string)
	}
	dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = disk.ID
	return dv, nil
}

func (r *Builder) getStorageClass() string {
	if r.Context.Map.Storage != nil {
		for _, pair := range r.Context.Map.Storage.Spec.Map {
			return pair.Destination.StorageClass
		}
	}
	return ""
}

func (r *Builder) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, disk := range vm.Disks {
		mB := disk.Capacity / 0x100000
		tasks = append(tasks, &plan.Task{
			Name: disk.ID,
			Progress: libitr.Progress{
				Total: mB,
			},
			Annotations: map[string]string{
				"unit": "MB",
			},
		})
	}
	return
}

func (r *Builder) TemplateLabels(_ ref.Ref) (labels map[string]string, err error) {
	labels = make(map[string]string)
	return
}

func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	if dv.ObjectMeta.Annotations != nil {
		if id, ok := dv.ObjectMeta.Annotations[planbase.AnnDiskSource]; ok {
			return id
		}
	}
	return dv.Name
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc.Annotations != nil {
		if id, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
			return id
		}
	}
	return pvc.Name
}

func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	var diskPaths []string
	for _, disk := range vm.Disks {
		if disk.SMBPath != "" {
			diskPaths = append(diskPaths, disk.SMBPath)
		}
	}

	env = append(env,
		core.EnvVar{Name: "V2V_vmName", Value: vm.Name},
		core.EnvVar{Name: "V2V_source", Value: "hyperv"},
		core.EnvVar{Name: "V2V_diskPath", Value: strings.Join(diskPaths, ",")},
	)

	if r.Plan.Spec.PreserveStaticIPs {
		macsToIps := r.mapMacStaticIps(vm)
		if macsToIps != "" {
			env = append(env,
				core.EnvVar{Name: "V2V_staticIPs", Value: macsToIps},
			)
		}
		if hasMultipleStaticIPsPerNIC(vm) {
			env = append(env, core.EnvVar{
				Name:  "V2V_multipleIPsPerNic",
				Value: "true",
			})
		}
	}

	if vm.Firmware == UEFI {
		env = append(env, core.EnvVar{Name: "V2V_firmware", Value: "uefi"})
	}

	return
}

func hasMultipleStaticIPsPerNIC(vm *model.VM) bool {
	if !isWindows(vm) {
		return false
	}
	macIPCount := make(map[string]int)
	for _, gn := range vm.GuestNetworks {
		if gn.Origin == hyperv.OriginManual && net.ParseIP(gn.IP).To4() != nil {
			macIPCount[gn.MAC]++
		}
	}
	for _, count := range macIPCount {
		if count > 1 {
			return true
		}
	}
	return false
}

func (r *Builder) mapMacStaticIps(vm *model.VM) string {
	isWin := isWindows(vm)

	var configurations []string
	for _, gn := range vm.GuestNetworks {
		if !isWin || gn.Origin == hyperv.OriginManual {
			ip := net.ParseIP(gn.IP)
			if ip == nil {
				continue
			}
			// Skip link-local addresses
			if ip.IsLinkLocalUnicast() {
				continue
			}
			// For Windows, skip IPv6
			if isWin && ip.To4() == nil {
				continue
			}

			dnsString := strings.Join(gn.DNS, ",")
			configurationString := fmt.Sprintf("%s:ip:%s,%s,%d,%s",
				gn.MAC, gn.IP, gn.Gateway, gn.PrefixLength, dnsString)

			// if DNS is "", we get configurationString with trailing comma, use TrimSuffix to remove it.
			configurations = append(configurations, strings.TrimSuffix(configurationString, ","))
		}
	}
	return strings.Join(configurations, "_")
}

func isWindows(vm *model.VM) bool {
	guestOS := strings.ToLower(vm.GuestOS)
	if strings.Contains(guestOS, "windows") {
		return true
	}
	// Fallback to name-based detection only if guestOS is empty/unknown
	if guestOS == "" {
		name := strings.ToLower(vm.Name)
		return strings.Contains(name, "win")
	}
	return false
}

func (r *Builder) LunPersistentVolumes(_ ref.Ref) (pvs []core.PersistentVolume, err error) {
	return
}

func (r *Builder) LunPersistentVolumeClaims(_ ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	return
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(_ ref.Ref, _ map[string]string, _ string) ([]*core.PersistentVolumeClaim, error) {
	return nil, planbase.VolumePopulatorNotSupportedError
}

func (r *Builder) PopulatorTransferredBytes(_ *core.PersistentVolumeClaim) (int64, error) {
	return 0, planbase.VolumePopulatorNotSupportedError
}

func (r *Builder) PopulatorOffloadInfo(_ *core.PersistentVolumeClaim) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(_ ref.Ref, _ []*core.PersistentVolumeClaim) error {
	return nil
}

func (r *Builder) GetPopulatorTaskName(_ *core.PersistentVolumeClaim) (string, error) {
	return "", nil
}

func (r *Builder) PreferenceName(_ ref.Ref, _ *core.ConfigMap) (string, error) {
	// HyperV doesn't have OS preference mapping configured
	// Users can manually specify instance types if needed
	return "", nil
}

func (r *Builder) ConfigMaps(_ ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(_ ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}

func (r *Builder) NetAppShiftPVCs(vmRef ref.Ref, labels map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) CsiImportPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(vmRef ref.Ref, tagMapping *api.TagMapping) (labels map[string]string, annotations map[string]string, sanitizationReport map[string]string, err error) {
	return
}
