package ova

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BIOS types
const (
	Efi = "efi"
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
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	Unknown        = "unknown"
)

// Annotations
const (
	// CDI import backing file annotation on PVC
	AnnImportBackingFile = "cdi.kubevirt.io/storage.import.backingFile"
)

// Map of vmware guest ids to osinfo ids.
var osMap = map[string]string{
	"centos64Guest":         "centos5.11",
	"centos6_64Guest":       "centos6.10",
	"centos6Guest":          "centos6.10",
	"centos7_64Guest":       "centos7.0",
	"centos7Guest":          "centos7.0",
	"centos8_64Guest":       "centos8",
	"centos8Guest":          "centos8",
	"debian4_64Guest":       "debian4",
	"debian4Guest":          "debian4",
	"debian5_64Guest":       "debian5",
	"debian5Guest":          "debian5",
	"debian6_64Guest":       "debian6",
	"debian6Guest":          "debian6",
	"debian7_64Guest":       "debian7",
	"debian7Guest":          "debian7",
	"debian8_64Guest":       "debian8",
	"debian8Guest":          "debian8",
	"debian9_64Guest":       "debian9",
	"debian9Guest":          "debian9",
	"debian10_64Guest":      "debian10",
	"debian10Guest":         "debian10",
	"fedora64Guest":         "fedora31",
	"fedoraGuest":           "fedora31",
	"genericLinuxGuest":     "linux",
	"rhel6_64Guest":         "rhel6.10",
	"rhel6Guest":            "rhel6.10",
	"rhel7_64Guest":         "rhel7.7",
	"rhel7Guest":            "rhel7.7",
	"rhel8_64Guest":         "rhel8.1",
	"ubuntu64Guest":         "ubuntu18.04",
	"ubuntuGuest":           "ubuntu18.04",
	"win2000AdvServGuest":   "win2k",
	"win2000ProGuest":       "win2k",
	"win2000ServGuest":      "win2k",
	"windows7Guest":         "win7",
	"windows7Server64Guest": "win2k8r2",
	"windows8_64Guest":      "win8",
	"windows8Guest":         "win8",
	"windows8Server64Guest": "win2k12r2",
	"windows9_64Guest":      "win10",
	"windows9Guest":         "win10",
	"windows9Server64Guest": "win2k19",
}

// Regex which matches the snapshot identifier suffix of a
// vSphere disk backing file.
var backingFilePattern = regexp.MustCompile("-\\d\\d\\d\\d\\d\\d.vmdk")

// vSphere builder.
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
			"VM lookup failed.",
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
			Value: "/mnt/nfs/centos44_new.ova",
		},
		core.EnvVar{
			Name:  "V2V_provider",
			Value: "ova",
		},
	)
	return
}

// Build the DataVolume credential secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	thumbprint := string(in.Data["thumbprint"])
	object.StringData = map[string]string{
		"accessKeyId": string(in.Data["user"]),
		"secretKey":   string(in.Data["password"]),
		"thumbprint":  thumbprint,
	}

	return
}

// Create DataVolume specs for the VM.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, _ *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	dsMapIn := r.Context.Map.Storage.Spec.Map
	for i := range dsMapIn {
		mapped := &dsMapIn[i]
		ref := mapped.Source
		ds := &model.Datastore{}
		fErr := r.Source.Inventory.Find(ds, ref)
		if fErr != nil {
			err = fErr
			return
		}
		for _, disk := range vm.Disks {
			if disk.Datastore.ID == ds.ID {
				storageClass := mapped.Destination.StorageClass
				var dvSource cdi.DataVolumeSource
				// Let virt-v2v do the copying
				dvSource = cdi.DataVolumeSource{
					Blank: &cdi.DataVolumeBlankImage{},
				}
				dvSpec := cdi.DataVolumeSpec{
					Source: &dvSource,
					Storage: &cdi.StorageSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: *resource.NewQuantity(disk.Capacity, resource.BinarySI),
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
				dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = trimBackingFileName(disk.File)
				dvs = append(dvs, *dv)
			}
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
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	if vm.IsTemplate {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s is a template",
				vmRef.String()))
		return
	}
	if types.VirtualMachineConnectionState(vm.ConnectionState) != types.VirtualMachineConnectionStateConnected {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s is not connected",
				vmRef.String()))
		return
	}
	if r.Plan.Spec.Warm && !vm.ChangeTrackingEnabled {
		err = liberr.New(
			fmt.Sprintf(
				"Changed Block Tracking (CBT) is disabled for VM %s",
				vmRef.String()))
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
	r.mapMemory(vm, object)
	r.mapClock(object)
	r.mapInput(object)
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

		needed := []vsphere.NIC{}
		for _, nic := range vm.NICs {
			switch network.Variant {
			case vsphere.NetDvPortGroup:
				if nic.Network.ID == network.Key {
					needed = append(needed, nic)
				}
			default:
				if nic.Network.ID == network.ID {
					needed = append(needed, nic)
				}
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

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec) {
	memoryBytes := int64(vm.MemoryMB) * 1024 * 1024
	reservation := resource.NewQuantity(memoryBytes, resource.BinarySI)
	object.Template.Spec.Domain.Resources = cnv.ResourceRequirements{
		Requests: map[core.ResourceName]resource.Quantity{
			core.ResourceMemory: *reservation,
		},
	}
}

func (r *Builder) mapCPU(vm *model.VM, object *cnv.VirtualMachineSpec) {
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
	case Efi:
		// We don't distinguish between UEFI and UEFI with secure boot but we anyway would have
		// disabled secure boot, even if we knew it was enabled on the source, because the guest
		// OS won't be able to boot without getting the NVRAM data. By starting the VM without
		// secure boot we ease the procedure users need to do in order to make a guest OS that
		// was previously configured with secure boot to boot.
		secureBootEnabled := false
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBootEnabled,
			}}
	default:
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Features = features
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapDisks(vm *model.VM, persistentVolumeClaims []core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	disks := vm.Disks
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Key < disks[j].Key
	})
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
		pvc := pvcMap[trimBackingFileName(disk.File)]
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
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}
	for _, disk := range vm.Disks {
		mB := disk.Capacity / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: trimBackingFileName(disk.File),
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
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	var os string
	if vm.GuestID != "" {
		os = osMap[vm.GuestID]
	} else if strings.Contains(vm.GuestName, "linux") || strings.Contains(vm.GuestName, "rhel") {
		os = DefaultLinux
	} else if strings.Contains(vm.GuestName, "win") {
		os = DefaultWindows
	} else {
		os = Unknown
	}

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

// Find host ID for VM.
func (r *Builder) hostID(vmRef ref.Ref) (hostID string, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	hostID = vm.Host

	return
}

// Find host CR secret.
func (r *Builder) hostSecret(host *api.Host) (secret *core.Secret, err error) {
	ref := host.Spec.Secret
	secret = &core.Secret{}
	err = r.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		secret)
	err = liberr.Wrap(err)

	return
}

// Find host in the inventory.
func (r *Builder) host(hostID string) (host *model.Host, err error) {
	host = &model.Host{}
	err = r.Source.Inventory.Get(host, hostID)
	if err != nil {
		err = liberr.Wrap(
			err,
			"Host lookup failed.",
			"host",
			hostID)
	}

	return
}

// Trims the snapshot suffix from a disk backing file name if there is one.
//
//	Example:
//	Input: 	[datastore13] my-vm/disk-name-000015.vmdk
//	Output: [datastore13] my-vm/disk-name.vmdk
func trimBackingFileName(fileName string) string {
	return backingFilePattern.ReplaceAllString(fileName, ".vmdk")
}

func (r *Builder) PersistentVolumeClaimWithSourceRef(da interface{}, storageName *string, populatorName string, accessModes []core.PersistentVolumeAccessMode, volumeMode *core.PersistentVolumeMode) *core.PersistentVolumeClaim {
	return nil
}

func (r *Builder) PreTransferActions(c planbase.Client, vmRef ref.Ref) (ready bool, err error) {
	return true, nil
}
