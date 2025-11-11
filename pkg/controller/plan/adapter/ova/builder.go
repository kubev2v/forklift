package ova

import (
	"fmt"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ova"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Firmware types
const (
	BIOS = "bios"
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

// OVA builder.
type Builder struct {
	*plancontext.Context
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
		err = liberr.Wrap(err, "vm", vmRef.String())
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
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	storageMapIn := r.Context.Map.Storage.Spec.Map
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
				dv, err = r.mapDataVolume(disk, mapped.Destination, dvTemplate)
				if err != nil {
					return
				}
				dvs = append(dvs, *dv)
			}
		}
	}

	return
}

func (r *Builder) mapDataVolume(disk ova.Disk, destination v1beta1.DestinationStorage, dvTemplate *cdi.DataVolume) (dv *cdi.DataVolume, err error) {
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
	// set the access mode and volume mode if they were specified in the storage map.
	// otherwise, let the storage profile decide the default values.
	if destination.AccessMode != "" {
		dvSpec.Storage.AccessModes = []core.PersistentVolumeAccessMode{destination.AccessMode}
	}
	if destination.VolumeMode != "" {
		dvSpec.Storage.VolumeMode = &destination.VolumeMode
	}

	dv = dvTemplate.DeepCopy()
	dv.Spec = dvSpec
	updateDataVolumeAnnotations(dv, &disk)
	return
}

func updateDataVolumeAnnotations(dv *cdi.DataVolume, disk *ova.Disk) {
	if dv.ObjectMeta.Annotations == nil {
		dv.ObjectMeta.Annotations = make(map[string]string)
	}
	dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = getDiskFullPath(disk)
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
	r.mapInput(object)
	if !usesInstanceType {
		r.mapCPU(vm, object)
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
	netMapIn := r.Context.Map.Network.Spec.Map
	for i := range netMapIn {
		mapped := &netMapIn[i]

		// Skip network mappings with destination type 'Ignored'
		if mapped.Destination.Type == Ignored {
			continue
		}

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

func (r *Builder) mapCPU(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.CoresPerSocket == 0 {
		vm.CoresPerSocket = 1
	}

	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.CpuCount / vm.CoresPerSocket),
		Cores:   uint32(vm.CoresPerSocket),
	}
}

func (r *Builder) mapFirmware(vm *model.VM, vmRef ref.Ref, object *cnv.VirtualMachineSpec) {
	var virtV2VFirmware string
	if vm.Firmware == "" {
		for _, vmConf := range r.Migration.Status.VMs {
			if vmConf.ID == vmRef.ID {
				virtV2VFirmware = vmConf.Firmware
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
	case BIOS:
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	default:
		// For UEFI firmware, use the SecureBoot value from the VM
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
	// We currently set the operating systems for VMs from OVA to UNKNOWN so we cannot get the corresponding preference
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
	return trimBackingFileName(dv.ObjectMeta.Annotations[planbase.AnnDiskSource])
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

func getDiskFullPath(disk *ova.Disk) string {
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

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}
