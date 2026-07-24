package nutanix

import (
	"fmt"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Firmware boot types (model.VM.BootType).
const (
	bootTypeLegacy     = "LEGACY"
	bootTypeUEFI       = "UEFI"
	bootTypeSecureBoot = "SECURE_BOOT"
)

// Disk adapter types (model.Disk.AdapterType).
const (
	adapterSCSI = "SCSI"
	adapterSATA = "SATA"
	adapterIDE  = "IDE"
)

type Builder struct {
	*plancontext.Context
}

func (r *Builder) Secret(_ ref.Ref, _, _ *core.Secret) error {
	return nil
}

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
	r.mapTpm(object)
	r.mapNetworks(vm, object)
	r.mapCPU(vmRef, vm, object, usesInstanceType)
	r.mapMemory(vm, object, usesInstanceType)

	return nil
}

func (r *Builder) mapDisks(vm *model.VM, pvcs []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	bootDiskIndex := -1
	for _, disk := range vm.Disks {
		if disk.IsCdrom {
			continue
		}
		if bootDiskIndex == -1 || disk.DeviceIndex < bootDiskIndex {
			bootDiskIndex = disk.DeviceIndex
		}
	}

	for _, disk := range vm.Disks {
		if disk.IsCdrom {
			continue
		}
		pvc := r.findPVC(disk.UUID, pvcs)
		if pvc == nil {
			r.Log.Info("PVC not found for disk, skipping",
				"diskID", disk.UUID,
				"vmName", vm.Name)
			continue
		}
		volumeName := disk.UUID
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
		kDisk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: diskBus(disk.AdapterType),
				},
			},
			Serial: disk.UUID,
		}
		if disk.DeviceIndex == bootDiskIndex {
			var bootOrder uint = 1
			kDisk.BootOrder = &bootOrder
		}
		kDisks = append(kDisks, kDisk)
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

// diskBus maps a Nutanix disk adapter type to a KubeVirt disk bus. AHV's
// SCSI and SATA/IDE adapters correspond directly to the equivalent KubeVirt
// buses; everything else (e.g. PCI) defaults to virtio, which AHV also
// exposes as a PCI-attached device.
func diskBus(adapterType string) cnv.DiskBus {
	switch adapterType {
	case adapterSCSI:
		return cnv.DiskBusSCSI
	case adapterSATA, adapterIDE:
		return cnv.DiskBusSATA
	default:
		return cnv.DiskBusVirtio
	}
}

// mapFirmware maps AHV's BootType (LEGACY, UEFI, SECURE_BOOT) to a KubeVirt
// firmware/bootloader configuration. SECURE_BOOT implies UEFI plus the SMM
// feature required for OVMF secure boot on KubeVirt.
func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	firmware := &cnv.Firmware{}
	switch vm.BootType {
	case bootTypeUEFI, bootTypeSecureBoot:
		secureBoot := vm.BootType == bootTypeSecureBoot
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBoot,
			},
		}
		if secureBoot {
			object.Template.Spec.Domain.Features = &cnv.Features{
				SMM: &cnv.FeatureState{
					Enabled: &secureBoot,
				},
			}
		}
	case bootTypeLegacy:
		fallthrough
	default:
		// Unrecognized boot types fall back to BIOS/legacy rather than
		// failing the migration outright.
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

// mapTpm explicitly disables the vTPM device. AHV VMs may have a vTPM
// configured, but that isn't modeled in Nutanix inventory yet, so there's
// nothing to map -- explicitly disabling avoids inheriting an unrelated
// default from an instance type or preference.
func (r *Builder) mapTpm(object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Enabled: ptr.To(false)}
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
	pool := planbase.NewNADPool()

	for _, nic := range vm.NICs {
		nicRef := ref.Ref{ID: nic.SubnetUUID}
		pairs := planbase.FindAllMappingsForNICRef(nicRef, r.Map.Network)
		mapped, allocated := planbase.AllocateNetwork(pool, pairs)

		// Skip if no valid mapping found or the destination type is Ignored.
		if !allocated || mapped.Destination.Type == planbase.Ignored {
			continue
		}

		networkName := fmt.Sprintf("net-%d", numNetworks)
		numNetworks++

		kNetwork := cnv.Network{Name: networkName}
		kInterface := cnv.Interface{
			Name:       networkName,
			MacAddress: nic.MACAddress,
		}

		switch mapped.Destination.Type {
		case planbase.Multus:
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
			}
			kInterface.Bridge = &cnv.InterfaceBridge{}
		case planbase.Pod:
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

func (r *Builder) mapCPU(vmRef ref.Ref, vm *model.VM, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.NumSockets),
		Cores:   uint32(vm.NumVcpusPerSocket),
		Threads: uint32(vm.NumThreadsPerCore),
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
	memory := resource.NewQuantity(vm.MemorySizeMiB*1024*1024, resource.BinarySI)
	object.Template.Spec.Domain.Resources.Requests = core.ResourceList{
		core.ResourceMemory: *memory,
	}
}

func (r *Builder) DataVolumes(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap, _ *cdi.DataVolume, _ *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	// TODO: build CDI HTTP import DataVolumes from catalog image file URLs
	return nil, nil
}

func (r *Builder) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, disk := range vm.Disks {
		if disk.IsCdrom {
			continue
		}
		mB := disk.DiskSizeBytes / 0x100000
		tasks = append(tasks, &plan.Task{
			Name: disk.UUID,
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
	if dv == nil || dv.Annotations == nil {
		return ""
	}
	return dv.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc == nil || pvc.Annotations == nil {
		return ""
	}
	return pvc.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) PodEnvironment(_ ref.Ref, _ *core.Secret) (env []core.EnvVar, err error) {
	return nil, nil
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

func (r *Builder) PopulatorXcopyUsed(_ *core.PersistentVolumeClaim) (string, bool, error) {
	return "", false, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(_ ref.Ref, _ []*core.PersistentVolumeClaim) error {
	return nil
}

func (r *Builder) GetPopulatorTaskName(_ *core.PersistentVolumeClaim) (string, error) {
	return "", nil
}

func (r *Builder) PreferenceName(_ ref.Ref, _ *core.ConfigMap) (string, error) {
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

func (r *Builder) NetAppShiftPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) CsiImportPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(_ ref.Ref, _ *api.TagMapping) (labels map[string]string, annotations map[string]string, sanitizationReport map[string]string, err error) {
	// TODO: map Nutanix categories to destination labels/annotations
	return
}
