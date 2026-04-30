package hyperv

import (
	"context"
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
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// AnnDefaultStorageClass is the standard annotation marking a StorageClass as the cluster default.
const AnnDefaultStorageClass = "storageclass.kubernetes.io/is-default-class"

type populatorCRParams struct {
	vm           *model.VM
	disk         hyperv.Disk
	diskIndex    int
	secretName   string
	vmID         string
	migrationUID string
	targetIQN    string
	portal       string
	initiatorIQN string
}

type pvcParams struct {
	disk          hyperv.Disk
	storageClass  string
	populatorName string
	annotations   map[string]string
	vmID          string
	migrationUID  string
}

type Builder struct {
	*plancontext.Context
}

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
		case Multus:
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
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
	storageClass, err := r.getStorageClass()
	if err != nil {
		return nil, err
	}
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

func (r *Builder) getStorageClass() (string, error) {
	if r.Context.Map.Storage != nil {
		for _, pair := range r.Context.Map.Storage.Spec.Map {
			return pair.Destination.StorageClass, nil
		}
	}
	scList := &storage.StorageClassList{}
	if err := r.Client.List(context.TODO(), scList); err != nil {
		return "", fmt.Errorf("list StorageClasses: %w", err)
	}
	for _, sc := range scList.Items {
		if sc.Annotations[AnnDefaultStorageClass] == "true" {
			return sc.Name, nil
		}
	}
	return "", fmt.Errorf("no storage class found: storage map is empty and no default StorageClass exists in the cluster")
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

	if r.Source.Provider.GetHyperVTransferMethod() == api.HyperVTransferMethodSMB {
		var diskPaths []string
		for _, disk := range vm.Disks {
			if disk.SMBPath != "" {
				diskPaths = append(diskPaths, disk.SMBPath)
			}
		}
		env = append(env,
			core.EnvVar{Name: "V2V_diskPath", Value: strings.Join(diskPaths, ",")},
		)
	}

	env = append(env,
		core.EnvVar{Name: "V2V_vmName", Value: vm.Name},
		core.EnvVar{Name: "V2V_source", Value: "hyperv"},
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
	return r.Source.Provider.GetHyperVTransferMethod() == api.HyperVTransferMethodISCSI
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	vm := &model.VM{}
	if err = r.Source.Inventory.Find(vm, vmRef); err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	params := populatorCRParams{
		vm:           vm,
		secretName:   secretName,
		vmID:         vmRef.ID,
		migrationUID: string(r.Migration.UID),
		portal:       iscsiTargetPortal(r.Context),
		initiatorIQN: iscsiInitiatorIQN(r.Context),
	}

	params.targetIQN, err = r.getTargetIQN(vmRef.ID)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	sc, scErr := r.getStorageClass()
	if scErr != nil {
		err = liberr.Wrap(scErr)
		return
	}
	pvcP := pvcParams{
		storageClass: sc,
		annotations:  annotations,
		vmID:         params.vmID,
		migrationUID: params.migrationUID,
	}

	for i, disk := range vm.Disks {
		pvc, skip, diskErr := r.ensureDiskPopulator(disk, i, &params, &pvcP)
		if diskErr != nil {
			err = diskErr
			return
		}
		if skip {
			continue
		}
		pvcs = append(pvcs, pvc)
	}
	return
}

func (r *Builder) ensureDiskPopulator(disk hyperv.Disk, diskIndex int, params *populatorCRParams, pvcP *pvcParams) (*core.PersistentVolumeClaim, bool, error) {
	diskID := disk.ID

	if r.pvcExistsForDisk(diskID, params.migrationUID) {
		return nil, true, nil
	}

	populatorName, err := r.ensurePopulatorCR(disk, diskIndex, params)
	if err != nil {
		return nil, false, liberr.Wrap(err, "disk", diskID)
	}

	pvcP.disk = disk
	pvcP.populatorName = populatorName
	pvc, pErr := r.persistentVolumeClaimWithSourceRef(*pvcP)
	if pErr != nil {
		if k8serr.IsAlreadyExists(pErr) {
			return nil, true, nil
		}
		return nil, false, liberr.Wrap(pErr, "disk", diskID, "storageClass", pvcP.storageClass)
	}
	return pvc, false, nil
}

// ensurePopulatorCR reuses an existing CR if a previous reconcile created it
// but the PVC creation failed before completing; otherwise creates a new one.
func (r *Builder) ensurePopulatorCR(disk hyperv.Disk, diskIndex int, params *populatorCRParams) (string, error) {
	existing, getErr := r.getVolumePopulator(disk.ID, params.migrationUID)
	if getErr == nil {
		return existing.Name, nil
	}
	if !k8serr.IsNotFound(getErr) {
		return "", getErr
	}
	params.disk = disk
	params.diskIndex = diskIndex
	return r.createVolumePopulatorCR(*params)
}

func (r *Builder) getTargetIQN(vmID string) (string, error) {
	// Read the real IQN persisted by PreTransferActions on the Migration object.
	annKey := iscsiTargetIQNAnnKey(vmID)
	if r.Migration.Annotations != nil {
		if iqn, ok := r.Migration.Annotations[annKey]; ok && iqn != "" {
			return iqn, nil
		}
	}
	return "", fmt.Errorf("real iSCSI target IQN not found on migration annotation %q; PreTransferActions may not have run yet", annKey)
}

// pvcExistsForDisk returns true if a PVC for the given disk already exists.
func (r *Builder) pvcExistsForDisk(diskID, migrationUID string) bool {
	list := core.PersistentVolumeClaimList{}
	err := r.Destination.Client.List(context.TODO(), &list, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": migrationUID,
			"diskID":    diskID,
		}),
	})
	return err == nil && len(list.Items) > 0
}

func (r *Builder) getVolumePopulator(diskID, migrationUID string) (api.HyperVVolumePopulator, error) {
	list := api.HyperVVolumePopulatorList{}
	err := r.Destination.Client.List(context.TODO(), &list, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": migrationUID,
			"diskID":    diskID,
		}),
	})
	if err != nil {
		return api.HyperVVolumePopulator{}, liberr.Wrap(err)
	}
	if len(list.Items) == 0 {
		return api.HyperVVolumePopulator{},
			k8serr.NewNotFound(
				api.SchemeGroupVersion.WithResource("HyperVVolumePopulator").GroupResource(),
				diskID,
			)
	}
	if len(list.Items) > 1 {
		r.Log.Info("Multiple HyperVVolumePopulator CRs found for disk; using first",
			"diskID", diskID, "migration", migrationUID, "count", len(list.Items))
	}
	return list.Items[0], nil
}

func (r *Builder) createVolumePopulatorCR(p populatorCRParams) (string, error) {
	cr := &api.HyperVVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("hyperv-%s-%d-", p.vmID[:min(8, len(p.vmID))], p.diskIndex),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels: map[string]string{
				"vmID":      p.vmID,
				"migration": p.migrationUID,
				"diskID":    p.disk.ID,
			},
		},
		Spec: api.HyperVVolumePopulatorSpec{
			SecretName:   p.secretName,
			VMID:         p.vmID,
			VMName:       p.vm.Name,
			DiskIndex:    p.diskIndex,
			DiskPath:     p.disk.WindowsPath,
			TargetIQN:    p.targetIQN,
			TargetPortal: p.portal,
			LunID:        p.diskIndex,
			InitiatorIQN: p.initiatorIQN,
		},
	}

	if err := r.Destination.Client.Create(context.TODO(), cr, &client.CreateOptions{}); err != nil {
		if k8serr.IsAlreadyExists(err) {
			existing, getErr := r.getVolumePopulator(p.disk.ID, p.migrationUID)
			if getErr != nil {
				return "", liberr.Wrap(getErr)
			}
			return existing.Name, nil
		}
		return "", liberr.Wrap(err)
	}
	return cr.Name, nil
}

func (r *Builder) persistentVolumeClaimWithSourceRef(p pvcParams) (*core.PersistentVolumeClaim, error) {
	diskSize := p.disk.Capacity
	accessModes, volumeMode, err := r.getDefaultVolumeAndAccessMode(p.storageClass)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	diskSize = util.CalculateSpaceWithOverhead(diskSize, volumeMode)

	pvcAnnotations := make(map[string]string, len(p.annotations)+2)
	for k, v := range p.annotations {
		pvcAnnotations[k] = v
	}
	pvcAnnotations[planbase.AnnDiskSource] = p.disk.ID
	pvcAnnotations["copy-offload"] = p.disk.ID
	pvcAnnotations[planbase.AnnUsePopulator] = "false"

	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("hyperv-%s-", p.disk.ID[:min(8, len(p.disk.ID))]),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Annotations:  pvcAnnotations,
			Labels: map[string]string{
				"migration": p.migrationUID,
				"vmID":      p.vmID,
				"diskID":    p.disk.ID,
			},
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: core.VolumeResourceRequirements{
				Requests: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: *resource.NewQuantity(diskSize, resource.BinarySI),
				},
			},
			StorageClassName: &p.storageClass,
			VolumeMode:       volumeMode,
			DataSourceRef: &core.TypedObjectReference{
				APIGroup: &api.SchemeGroupVersion.Group,
				Kind:     api.HyperVVolumePopulatorKind,
				Name:     p.populatorName,
			},
		},
	}

	err = r.Destination.Client.Create(context.TODO(), pvc, &client.CreateOptions{})
	return pvc, err
}

func (r *Builder) getDefaultVolumeAndAccessMode(storageClassName string) ([]core.PersistentVolumeAccessMode, *core.PersistentVolumeMode, error) {
	filesystemMode := core.PersistentVolumeFilesystem
	storageProfile := &cdi.StorageProfile{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: storageClassName}, storageProfile)
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	if len(storageProfile.Status.ClaimPropertySets) > 0 &&
		len(storageProfile.Status.ClaimPropertySets[0].AccessModes) > 0 {
		accessModes := storageProfile.Status.ClaimPropertySets[0].AccessModes
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		if volumeMode == nil {
			volumeMode = &filesystemMode
		}
		return accessModes, volumeMode, nil
	}
	return nil, nil, liberr.New("no accessMode defined on StorageProfile for StorageClass", "storageName", storageClassName)
}

func (r *Builder) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (int64, error) {
	if pvc.Annotations == nil {
		return 0, nil
	}
	diskID := pvc.Annotations[planbase.AnnDiskSource]
	if diskID == "" {
		return 0, nil
	}
	migrationUID := string(r.Migration.UID)

	cr, err := r.getVolumePopulator(diskID, migrationUID)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return 0, nil
		}
		return 0, liberr.Wrap(err, "diskID", diskID)
	}

	if cr.Status.Progress == "" {
		return 0, nil
	}
	progressStr := strings.TrimSuffix(cr.Status.Progress, "%")
	pct, pErr := strconv.ParseInt(progressStr, 10, 64)
	if pErr != nil {
		r.Log.V(1).Info("Failed to parse populator progress",
			"progress", cr.Status.Progress, "error", pErr)
		return 0, nil
	}

	pvcSize := pvc.Spec.Resources.Requests[core.ResourceStorage]
	return (pct * pvcSize.Value()) / 100, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) error {
	migrationUID := string(r.Migration.UID)
	for _, pvc := range pvcs {
		diskID := pvc.Annotations[planbase.AnnDiskSource]
		if diskID == "" {
			continue
		}
		cr, err := r.getVolumePopulator(diskID, migrationUID)
		if err != nil {
			if !k8serr.IsNotFound(err) {
				r.Log.Error(err, "Failed to get volume populator for label update", "diskID", diskID)
			}
			continue
		}
		patch := client.MergeFrom(cr.DeepCopy())
		if cr.Labels == nil {
			cr.Labels = make(map[string]string)
		}
		cr.Labels["vmID"] = vmRef.ID
		cr.Labels["migration"] = migrationUID
		if pErr := r.Destination.Client.Patch(context.TODO(), &cr, patch); pErr != nil {
			return liberr.Wrap(pErr)
		}
	}
	return nil
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (string, error) {
	return pvc.Annotations[planbase.AnnDiskSource], nil
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
