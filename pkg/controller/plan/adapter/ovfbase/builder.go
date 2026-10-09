package ovfbase

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ovfmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
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
	EFI  = "efi"
	UEFI = "uefi"
)

// Bus types
const (
	Virtio = "virtio"
)

// Input types
const (
	Tablet = "tablet"
)

// Network types
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// Template labels
const (
	TemplateOSLabel       = "os.template.kubevirt.io/%s"
	TemplateWorkloadLabel = "workload.template.kubevirt.io/server"
	TemplateFlavorLabel   = "flavor.template.kubevirt.io/medium"
)

// Operating Systems
const (
	Unknown = "unknown"
)

// Regex which matches the snapshot identifier suffix of a
// OVA disk backing file.
var backingFilePattern = regexp.MustCompile(`-\d\d\d\d\d\d.vmdk`)

// Builder for OVF-based providers.
type Builder struct {
	*plancontext.Context
}

// Create DataVolume certificate configmap.
// No-op for OVF-based providers.
func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) (err error) {
	return
}

func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// virt-v2v -i ova reads the OVF file and finds the disk references
	env = append(
		env,
		core.EnvVar{
			Name:  "V2V_vmName",
			Value: vm.Name,
		},
		core.EnvVar{
			Name:  "V2V_diskPath",
			Value: getDiskSourcePath(vm.OvfPath),
		},
		core.EnvVar{
			Name:  "V2V_source",
			Value: "ova",
		})

	return
}

// Build the DataVolume credential secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	return
}

// Create DataVolume specs for the VM.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	diskIndex := 0
	storageMapIn := r.Map.Storage.Spec.Map
	for i := range storageMapIn {
		mapped := &storageMapIn[i]
		ref := mapped.Source
		storage := &model.Storage{}
		fErr := r.Source.Inventory.Find(storage, ref)
		if fErr != nil {
			err = fErr
			return
		}
		for _, disk := range vm.Disks {
			if disk.ID == storage.ID {
				var dv *cdi.DataVolume
				dv, err = r.mapDataVolume(vm, disk, mapped.Destination, diskIndex, dvTemplate)
				if err != nil {
					return
				}
				dvs = append(dvs, *dv)
				diskIndex++
			}
		}
	}

	return
}

func (r *Builder) mapDataVolume(vm *model.VM, disk ovfmodel.Disk, destination api.DestinationStorage, diskIndex int, dvTemplate *cdi.DataVolume) (dv *cdi.DataVolume, err error) {
	diskSize, err := getResourceCapacity(disk.Capacity, disk.CapacityAllocationUnits)
	if err != nil {
		return
	}
	storageClass := destination.StorageClass
	dvSource := cdi.DataVolumeSource{
		Blank: &cdi.DataVolumeBlankImage{},
	}
	dvSpec := cdi.DataVolumeSpec{
		Source: &dvSource,
		Storage: &cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: *resource.NewQuantity(diskSize, resource.BinarySI),
				},
			},
			StorageClassName: &storageClass,
		},
	}
	if destination.AccessMode != "" {
		dvSpec.Storage.AccessModes = []core.PersistentVolumeAccessMode{destination.AccessMode}
	}
	if destination.VolumeMode != "" {
		dvSpec.Storage.VolumeMode = &destination.VolumeMode
	}

	dv = dvTemplate.DeepCopy()
	dv.Spec = dvSpec
	updateDataVolumeAnnotations(dv, &disk)

	templateData := &api.PVCNameTemplateData{
		VmName:       vm.Name,
		TargetVmName: planbase.ResolveTargetVmName(r.Plan, vm.ID, vm.Name),
		PlanName:     r.Plan.Name,
		DiskIndex:    diskIndex,
		VmId:         vm.ID,
	}
	pvcNameTemplate := planbase.GetPVCNameTemplate(r.Plan, vm.ID)
	if nameErr := planbase.SetPVCNameOnObject(&dv.ObjectMeta, pvcNameTemplate, planbase.GetPVCNameTemplateUseGenerateName(r.Plan), templateData); nameErr != nil {
		err = liberr.Wrap(nameErr, "vm", vm.ID, "diskIndex", diskIndex)
	}
	return
}

func updateDataVolumeAnnotations(dv *cdi.DataVolume, disk *ovfmodel.Disk) {
	if dv.Annotations == nil {
		dv.Annotations = make(map[string]string)
	}
	dv.Annotations[planbase.AnnDiskSource] = getDiskFullPath(disk)
}

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) (err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapFirmware(vm, vmRef, object)
	r.mapTpm(vm, object)
	r.mapMachine(vm, object)
	r.mapClock(vm, object)
	r.mapInput(object)
	if !usesInstanceType {
		r.mapCPU(vmRef, vm, object)
		err = r.mapMemory(vm, object)
		if err != nil {
			return
		}
	}
	err = r.mapNetworks(vm, object)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)

	resolved, rErr := resolveNICMappings(vm.NICs, r.Map.Network.Spec.Map, r.Source.Inventory)
	if rErr != nil {
		err = rErr
		return
	}

	for _, entry := range resolved {
		nic := entry.NIC
		mapped := entry.Mapping

		networkName := fmt.Sprintf("net-%v", numNetworks)
		numNetworks++
		kNetwork := cnv.Network{
			Name: networkName,
		}
		kInterface := cnv.Interface{
			Name:  networkName,
			Model: Virtio,
		}
		if !hasUDN || settings.Settings.UdnSupportsMac {
			kInterface.MacAddress = nic.MAC
		}
		switch mapped.Destination.Type {
		case Pod:
			kNetwork.Pod = &cnv.PodNetwork{}
			if hasUDN {
				kInterface.Binding = &cnv.PluginBinding{
					Name: planbase.UdnL2bridge,
				}
			} else {
				kInterface.Masquerade = &cnv.InterfaceMasquerade{}
			}
		case Multus:
			networkName := planbase.QualifiedMultusNetworkName(mapped.Destination.Namespace, mapped.Destination.Name, r.Plan.Spec.TargetNamespace)
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: networkName,
			}
			kInterface.Bridge = &cnv.InterfaceBridge{}
		}
		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	}
	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces
	return
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	tablet := cnv.Input{
		Type: Tablet,
		Name: Tablet,
		Bus:  Virtio,
	}
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{tablet}
}

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec) error {
	var memoryBytes int64
	memoryBytes, err := getResourceCapacity(int64(vm.MemoryMB), vm.MemoryUnits)
	if err != nil {
		return err
	}
	reservation := resource.NewQuantity(memoryBytes, resource.BinarySI)
	object.Template.Spec.Domain.Memory = &cnv.Memory{Guest: reservation}
	return nil
}

// mapCPU sets KubeVirt CPU topology from inventory, preferring a complete
// Nutanix sockets×cores×threads description when it matches CpuCount.
func (r *Builder) mapCPU(vmRef ref.Ref, vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.CoresPerSocket == 0 {
		vm.CoresPerSocket = 1
	}
	threads := uint32(1)
	if vm.ThreadsPerCore > 0 {
		threads = uint32(vm.ThreadsPerCore)
	}

	sockets := uint32(0)
	if vm.CpuCount > 0 {
		sockets = uint32(vm.CpuCount / vm.CoresPerSocket)
	}
	// Prefer Nutanix num_sockets only when it is consistent with CpuCount
	// (or CpuCount is unset). Incomplete topology must not shrink vCPUs.
	if vm.NumSockets > 0 {
		expected := int64(vm.NumSockets) * int64(vm.CoresPerSocket) * int64(threads)
		if vm.CpuCount == 0 || int64(vm.CpuCount) == expected {
			sockets = uint32(vm.NumSockets)
		}
	}

	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: sockets,
		Cores:   uint32(vm.CoresPerSocket),
		Threads: threads,
	}
	nestedDefault := vm.NestedVirtualization
	if enableNestedVirt := r.NestedVirtualizationSetting(vmRef, nestedDefault); enableNestedVirt != nil {
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

// mapTpm enables a persistent TPM when inventory reports vTPM, otherwise disables it.
func (r *Builder) mapTpm(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.TpmEnabled {
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Persistent: ptr.To(true)}
		return
	}
	object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Enabled: ptr.To(false)}
}

// mapMachine sets Domain.Machine for supported QEMU types (q35 / pc-q35*).
// Unsupported values such as Nutanix "pc" are left unset for the KubeVirt default.
func (r *Builder) mapMachine(vm *model.VM, object *cnv.VirtualMachineSpec) {
	mt := strings.ToLower(strings.TrimSpace(vm.MachineType))
	if mt == "" {
		return
	}
	if mt != "q35" && !strings.HasPrefix(mt, "pc-q35") {
		return
	}
	object.Template.Spec.Domain.Machine = &cnv.Machine{Type: mt}
}

// mapClock sets the guest clock from HardwareClockTimezone when present.
func (r *Builder) mapClock(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.HardwareClockTimezone == "" {
		return
	}
	if object.Template.Spec.Domain.Clock == nil {
		object.Template.Spec.Domain.Clock = &cnv.Clock{}
	}
	if strings.EqualFold(vm.HardwareClockTimezone, "UTC") {
		object.Template.Spec.Domain.Clock.UTC = &cnv.ClockOffsetUTC{}
		return
	}
	tz := cnv.ClockOffsetTimezone(vm.HardwareClockTimezone)
	object.Template.Spec.Domain.Clock.Timezone = &tz
}

func (r *Builder) mapFirmware(vm *model.VM, vmRef ref.Ref, object *cnv.VirtualMachineSpec) {
	var virtV2VFirmware string
	if vm.Firmware == "" {
		// If the VM model doesn't have firmware info
		for _, vmConf := range r.Migration.Status.VMs {
			if vmConf.ID == vmRef.ID {
				virtV2VFirmware = vmConf.Firmware // Get it from virt-v2v output
				break
			}
		}
		if virtV2VFirmware == "" {
			r.Log.Info("failed to match the vm", "model ID", vm.ID, "vmRef ID", vmRef.ID)
		}
	} else {
		virtV2VFirmware = vm.Firmware
	}

	firmware := &cnv.Firmware{
		Serial: vm.UUID,
	}

	switch virtV2VFirmware {
	case EFI, UEFI:
		// For EFI/UEFI firmware, use the SecureBoot value from the VM
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &vm.SecureBoot,
			}}
		if vm.SecureBoot {
			object.Template.Spec.Domain.Features = &cnv.Features{
				SMM: &cnv.FeatureState{
					Enabled: &vm.SecureBoot,
				},
			}
		}
	default:
		// BIOS or unknown/empty - default to BIOS (matches vSphere behavior)
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapDisks(vm *model.VM, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	disks := vm.Disks
	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i := range persistentVolumeClaims {
		pvc := persistentVolumeClaims[i]
		if source, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
			pvcMap[source] = pvc
		}
	}
	bootDiskPath := ovaBootDiskPath(vm.BootDeviceOrder, disks)
	for i, disk := range disks {
		pvc := pvcMap[getDiskFullPath(&disk)]
		volumeName := fmt.Sprintf("vol-%v", i)
		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}
		kubevirtDisk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: Virtio,
				},
			},
			Serial: planbase.DiskSerial(disk.DiskId, vm.ID, i),
		}
		if bootDiskPath != "" && getDiskFullPath(&disk) == bootDiskPath {
			kubevirtDisk.BootOrder = ptr.To(uint(1))
		}
		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, kubevirtDisk)
	}
	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

// ovaBootDiskPath returns the inventory path of the disk that should receive
// KubeVirt boot order when BootDeviceOrder lists DISK (Nutanix OVA). Empty
// BootDeviceOrder leaves boot order unset so VMware OVAs stay unchanged.
func ovaBootDiskPath(order string, disks []ovfmodel.Disk) string {
	if order == "" || len(disks) == 0 {
		return ""
	}
	for _, part := range strings.Split(order, ",") {
		switch strings.ToUpper(strings.TrimSpace(part)) {
		case "":
			continue
		case "DISK":
			return getDiskFullPath(&disks[0])
		default:
			// CDROM/NETWORK before DISK: do not promote the disk ahead of them.
			return ""
		}
	}
	return ""
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	for _, disk := range vm.Disks {
		mB := disk.Capacity / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: getDiskFullPath(&disk),
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

func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	// We currently set the operating systems for VMs from OVF-based providers to UNKNOWN so we cannot get the corresponding preference
	err = liberr.New("preferences are not used by this provider")
	return
}

func (r *Builder) ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(vmRef ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	os := Unknown

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return
}

func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return trimBackingFileName(dv.Annotations[planbase.AnnDiskSource])
}

// Return a stable identifier for a PersistentDataVolume.
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

// Trims the snapshot suffix from a disk backing file name if there is one.
//
//	Example:
//	Input: 	[datastore13] my-vm/disk-name-000015.vmdk
//	Output: [datastore13] my-vm/disk-name.vmdk
func trimBackingFileName(fileName string) string {
	return backingFilePattern.ReplaceAllString(fileName, ".vmdk")
}

func getDiskFullPath(disk *ovfmodel.Disk) string {
	return disk.FilePath + "::" + disk.Name
}

func getDiskSourcePath(filePath string) string {
	if strings.HasSuffix(filePath, ".ova") {
		return filePath
	}
	return filepath.Dir(filePath)
}

func getResourceCapacity(capacity int64, units string) (int64, error) {
	if strings.ToLower(units) == "megabytes" {
		return capacity * (1 << 20), nil
	}
	items := strings.Split(units, "*")
	for i := range items {
		item := strings.TrimSpace(items[i])
		if i == 0 && len(item) > 0 && item != "byte" {
			return 0, fmt.Errorf("units '%s' are invalid, only 'byte' is supported", units)
		}
		if i == 0 {
			continue
		}
		num, err := strconv.Atoi(item)
		if err == nil {
			capacity = capacity * int64(num)
			continue
		}
		nums := strings.Split(item, "^")
		if len(nums) != 2 {
			return 0, fmt.Errorf("units '%s' are invalid, item is invalid: %s", units, item)
		}
		base, err := strconv.Atoi(nums[0])
		if err != nil {
			return 0, fmt.Errorf("units '%s' are invalid, base component is invalid: %s", units, item)
		}
		pow, err := strconv.Atoi(nums[1])
		if err != nil {
			return 0, fmt.Errorf("units '%s' are invalid, pow component is invalid: %s", units, item)
		}
		capacity = capacity * int64(math.Pow(float64(base), float64(pow)))
	}
	return capacity, nil
}

// Build LUN PVs.
func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	// do nothing
	return
}

// Build LUN PVCs.
func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	// do nothing
	return
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PrePopulateActions(c planbase.Client, vmRef ref.Ref) (ready bool, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PopulatorOffloadInfo(_ *core.PersistentVolumeClaim) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

// ConversionPodConfig returns provider-specific configuration for the virt-v2v conversion pod.
// OVF/OVA provider does not require any special configuration.
func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}

func (r *Builder) NetAppShiftPVCs(vmRef ref.Ref, labels map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) CsiImportPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) AdoptDownloadCookieSecretOwner(_ *cdi.DataVolume) error {
	return nil
}

func (r *Builder) RefreshImportCredentials(_ *cdi.DataVolume) (bool, error) {
	return false, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(vmRef ref.Ref, tagMapping *api.TagMapping) (labels map[string]string, annotations map[string]string, sanitizationReport map[string]string, err error) {
	return
}

func (r *Builder) DomainXML(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (string, error) {
	return "", nil
}
