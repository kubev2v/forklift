package hyperv

import (
	"fmt"
	"net"
	"path"
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
	r.mapCPU(vm, object, usesInstanceType)
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
			Bus:  cnv.InputBusUSB,
		},
	}
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	var netMapIn []api.NetworkPair
	if r.Context.Map.Network != nil {
		netMapIn = r.Context.Map.Network.Spec.Map
	}

	for _, nic := range vm.NICs {
		mapped := r.findNetworkMapping(nic, netMapIn)

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
		case Pod:
			kNetwork.Pod = &cnv.PodNetwork{}
			kInterface.Masquerade = &cnv.InterfaceMasquerade{}
		case Multus:
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
			}
			kInterface.Bridge = &cnv.InterfaceBridge{}
		default:
			// Default to Pod network if type is unknown
			kNetwork.Pod = &cnv.PodNetwork{}
			kInterface.Masquerade = &cnv.InterfaceMasquerade{}
		}

		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces
}

func (r *Builder) findNetworkMapping(nic hyperv.NIC, netMap []api.NetworkPair) *api.NetworkPair {
	for i := range netMap {
		candidate := &netMap[i]
		network := &model.Network{}
		if err := r.Source.Inventory.Find(network, candidate.Source); err != nil {
			continue
		}

		// Match by network ID
		if nic.Network.ID == network.ID {
			return candidate
		}
	}
	return nil
}

func (r *Builder) mapCPU(vm *model.VM, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: 1,
		Cores:   uint32(vm.CpuCount),
		Threads: 1,
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

		// Check for multiple IPs per NIC on Windows
		if isWindows(vm) {
			macIPCount := make(map[string]int)
			for _, gn := range vm.GuestNetworks {
				// Only count manual (static) IPv4 addresses
				if gn.Origin == hyperv.OriginManual && net.ParseIP(gn.IP).To4() != nil {
					macIPCount[gn.MAC]++
				}
			}
			for _, count := range macIPCount {
				if count > 1 {
					env = append(env, core.EnvVar{
						Name:  "V2V_multipleIPsPerNic",
						Value: "true",
					})
					break
				}
			}
		}
	}

	if vm.Firmware == UEFI {
		env = append(env, core.EnvVar{Name: "V2V_firmware", Value: "uefi"})
	}

	return
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
