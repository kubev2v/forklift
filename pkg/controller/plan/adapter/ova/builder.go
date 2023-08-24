package ova

import (
	"errors"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ova"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// BIOS types
const (
	BIOS = "bios"
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
	Pod    = "pod"
	Multus = "multus"
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

// Annotations
const (
	// CDI import backing file annotation on PVC
	AnnImportBackingFile = "cdi.kubevirt.io/storage.import.backingFile"
)

// Error messages
const (
	ErrVMLookupFailed = "VM lookup failed."
)

// Regex which matches the snapshot identifier suffix of a
// OVA disk backing file.
var backingFilePattern = regexp.MustCompile("-\\d\\d\\d\\d\\d\\d.vmdk")

// OVA builder.
type Builder struct {
	*plancontext.Context
	// MAC addresses already in use on the destination cluster. k=mac, v=vmName
	macConflictsMap map[string]string
}

// Get list of destination VMs with mac addresses that would
// conflict with this VM, if any exist.
func (r *Builder) macConflicts(vm *model.VM) (conflictingVMs []string, err error) {
	if r.macConflictsMap == nil {
		list := []ocp.VM{}
		err = r.Destination.Inventory.List(&list, base.Param{
			Key:   base.DetailParam,
			Value: "all",
		})
		if err != nil {
			return
		}

		r.macConflictsMap = make(map[string]string)
		for _, kVM := range list {
			for _, iface := range kVM.Object.Spec.Template.Spec.Domain.Devices.Interfaces {
				r.macConflictsMap[iface.MacAddress] = path.Join(kVM.Namespace, kVM.Name)
			}
		}
	}

	for _, nic := range vm.NICs {
		if conflictingVm, found := r.macConflictsMap[nic.MAC]; found {
			for i := range conflictingVMs {
				// ignore duplicates
				if conflictingVMs[i] == conflictingVm {
					continue
				}
			}
			conflictingVMs = append(conflictingVMs, conflictingVm)
		}
	}

	return
}

// Create DataVolume certificate configmap.
// No-op for OVA.
func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) (err error) {
	return
}

func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			ErrVMLookupFailed,
			"vm",
			vmRef.String())
		return
	}

	env = append(
		env,
		core.EnvVar{
			Name:  "V2V_vmName",
			Value: vm.Name,
		},
		core.EnvVar{
			Name:  "V2V_diskPath",
			Value: getDiskSourcePath(vm.OvaPath),
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
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, _ *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			ErrVMLookupFailed,
			"vm",
			vmRef.String())
		return
	}

	// For OVA provider we are assuming a single storage mapping.
	dsMapIn := r.Context.Map.Storage.Spec.Map
	for _, mapped := range dsMapIn {
		for _, disk := range vm.Disks {
			diskSize, err := getResourceCapacity(disk.Capacity, disk.CapacityAllocationUnits)
			if err != nil {
				return nil, err
			}
			storageClass := mapped.Destination.StorageClass
			var dvSource cdi.DataVolumeSource
			dvSource = cdi.DataVolumeSource{
				Blank: &cdi.DataVolumeBlankImage{},
			}
			dvSpec := cdi.DataVolumeSpec{
				Source: &dvSource,
				Storage: &cdi.StorageSpec{
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: *resource.NewQuantity(diskSize, resource.BinarySI),
						},
					},
					StorageClassName: &storageClass,
				},
			}
			// set the access mode and volume mode if they were specified in the storage map.
			// otherwise, let the storage profile decide the default values.
			if mapped.Destination.AccessMode != "" {
				dvSpec.Storage.AccessModes = []core.PersistentVolumeAccessMode{mapped.Destination.AccessMode}
			}
			if mapped.Destination.VolumeMode != "" {
				dvSpec.Storage.VolumeMode = &mapped.Destination.VolumeMode
			}

			dv := dvTemplate.DeepCopy()
			dv.Spec = dvSpec
			if dv.ObjectMeta.Annotations == nil {
				dv.ObjectMeta.Annotations = make(map[string]string)
			}
			dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = getDiskSourcePath(disk.FilePath)
			dvs = append(dvs, *dv)
		}
	}

	return
}

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			ErrVMLookupFailed,
			"vm",
			vmRef.String())
		return
	}

	var conflicts []string
	conflicts, err = r.macConflicts(vm)
	if err != nil {
		return
	}
	if len(conflicts) > 0 {
		err = liberr.New(
			fmt.Sprintf("Source VM has a mac address conflict with one or more destination VMs: %s", conflicts))
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapFirmware(vm, object)
	r.mapCPU(vm, object)
	r.mapClock(object)
	r.mapInput(object)
	err = r.mapMemory(vm, object)
	if err != nil {
		return
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
	netMapIn := r.Context.Map.Network.Spec.Map
	for i := range netMapIn {
		mapped := &netMapIn[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Source.Inventory.Find(network, ref)
		if fErr != nil {
			err = fErr
			return
		}

		needed := []ova.NIC{}
		for _, nic := range vm.NICs {
			if nic.Network == network.Name {
				needed = append(needed, nic)
			}
		}
		if len(needed) == 0 {
			continue
		}
		for _, nic := range needed {
			networkName := fmt.Sprintf("net-%v", numNetworks)
			numNetworks++
			kNetwork := cnv.Network{
				Name: networkName,
			}
			kInterface := cnv.Interface{
				Name:       networkName,
				Model:      Virtio,
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
			}
			kNetworks = append(kNetworks, kNetwork)
			kInterfaces = append(kInterfaces, kInterface)
		}
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

func (r *Builder) mapClock(object *cnv.VirtualMachineSpec) {
	clock := &cnv.Clock{
		Timer: &cnv.Timer{},
	}
	object.Template.Spec.Domain.Clock = clock
}

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec) error {
	var memoryBytes int64
	memoryBytes, err := getResourceCapacity(int64(vm.MemoryMB), vm.MemoryUnits)
	if err != nil {
		return err
	}
	reservation := resource.NewQuantity(memoryBytes, resource.BinarySI)
	object.Template.Spec.Domain.Resources = cnv.ResourceRequirements{
		Requests: map[core.ResourceName]resource.Quantity{
			core.ResourceMemory: *reservation,
		},
	}
	return nil
}

func (r *Builder) mapCPU(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.CoresPerSocket == 0 {
		vm.CoresPerSocket = 1
	}
	object.Template.Spec.Domain.Machine = &cnv.Machine{Type: "q35"}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.CpuCount / vm.CoresPerSocket),
		Cores:   uint32(vm.CoresPerSocket),
	}
}

func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	features := &cnv.Features{}
	firmware := &cnv.Firmware{
		Serial: vm.UUID,
	}
	switch vm.Firmware {
	case BIOS:
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	default:
		// We don't distinguish between UEFI and UEFI with secure boot, but we anyway would have
		// disabled secure boot, even if we knew it was enabled on the source, because the guest
		// OS won't be able to boot without getting the NVRAM data. By starting the VM without
		// secure boot we ease the procedure users need to do in order to make a guest OS that
		// was previously configured with secure boot bootable.
		secureBootEnabled := false
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBootEnabled,
			}}
	}
	object.Template.Spec.Domain.Features = features
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapDisks(vm *model.VM, persistentVolumeClaims []core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	disks := vm.Disks
	// TODO might need sort by the disk id (incremental name)
	/*sort.Slice(disks, func(i, j int) bool {
		return disks[i].Key < disks[j].Key
	})*/
	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i := range persistentVolumeClaims {
		pvc := &persistentVolumeClaims[i]
		// the PVC BackingFile value has already been trimmed.
		if source, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
			pvcMap[source] = pvc
		} else {
			pvcMap[pvc.Annotations[AnnImportBackingFile]] = pvc
		}
	}
	for i, disk := range disks {
		pvc := pvcMap[getDiskSourcePath(disk.FilePath)]
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
		}
		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, kubevirtDisk)
	}
	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			ErrVMLookupFailed,
			"vm",
			vmRef.String())
		return
	}
	for _, disk := range vm.Disks {
		mB := disk.Capacity / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: getDiskSourcePath(disk.FilePath),
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

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			ErrVMLookupFailed,
			"vm",
			vmRef.String())
		return
	}

	os := Unknown

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return
}

// Return a stable identifier for a VDDK DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return trimBackingFileName(dv.ObjectMeta.Annotations[planbase.AnnDiskSource])
}

// Return a stable identifier for a PersistentDataVolume.
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return trimBackingFileName(pvc.Annotations[AnnImportBackingFile])
}

// Trims the snapshot suffix from a disk backing file name if there is one.
//
//	Example:
//	Input: 	[datastore13] my-vm/disk-name-000015.vmdk
//	Output: [datastore13] my-vm/disk-name.vmdk
func trimBackingFileName(fileName string) string {
	return backingFilePattern.ReplaceAllString(fileName, ".vmdk")
}

func getDiskSourcePath(filePath string) string {
	if strings.HasSuffix(filePath, ".ova") {
		return filePath
	}
	return filepath.Dir(filePath)
}

func getResourceCapacity(capacity int64, units string) (int64, error) {
	items := strings.Split(units, "*")
	for i := range items {
		item := strings.TrimSpace(items[i])
		if i == 0 && len(item) > 0 && item != "byte" {
			return 0, errors.New(fmt.Sprintf("units '%s' are invalid, only 'byte' is supported", units))
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
			return 0, errors.New(fmt.Sprintf("units '%s' are invalid, item is invalid: %s", units, item))
		}
		base, err := strconv.Atoi(nums[0])
		if err != nil {
			return 0, errors.New(fmt.Sprintf("units '%s' are invalid, base component is invalid: %s", units, item))
		}
		pow, err := strconv.Atoi(nums[1])
		if err != nil {
			return 0, errors.New(fmt.Sprintf("units '%s' are invalid, pow component is invalid: %s", units, item))
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

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcNames []string, err error) {
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

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}
