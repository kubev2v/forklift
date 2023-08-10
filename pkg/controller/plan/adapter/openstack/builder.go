package openstack

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	utils "github.com/konveyor/forklift-controller/pkg/controller/plan/util"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Openstack builder.
type Builder struct {
	*plancontext.Context
	// MAC addresses already in use on the destination cluster. k=mac, v=vmName
	macConflictsMap map[string]string
}

// Template labels
const (
	TemplateOSLabel                 = "os.template.kubevirt.io/%s"
	TemplateWorkloadLabel           = "workload.template.kubevirt.io/%s"
	TemplateWorkloadServer          = "server"
	TemplateWorkloadDesktop         = "desktop"
	TemplateWorkloadHighPerformance = "highperformance"
	TemplateFlavorLabel             = "flavor.template.kubevirt.io/%s"
	TemplateFlavorTiny              = "tiny"
	TemplateFlavorSmall             = "small"
	TemplateFlavorMedium            = "medium"
	TemplateFlavorLarge             = "large"
)

// Annotations
const (
	AnnImportDiskId = "cdi.kubevirt.io/storage.import.volumeId"
)

// OS types
const (
	Linux = "linux"
)

// OS Distros
const (
	ArchLinux   = "arch"
	CentOS      = "centos"
	Debian      = "debian"
	Fedora      = "fedora"
	FreeBSD     = "freebsd"
	Gentoo      = "gentoo"
	Mandrake    = "mandrake"
	Mandriva    = "mandriva"
	MES         = "mes"
	MSDOS       = "msdos"
	NetBSD      = "netbsd"
	Netware     = "netware"
	OpenBSD     = "openbsd"
	OpenSolaris = "opensolaris"
	OpenSUSE    = "opensuse"
	RHEL        = "rhel"
	SLED        = "sled"
	Ubuntu      = "ubuntu"
	Windows     = "windows"
)

// Default Operating Systems
const (
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	UnknownOS      = "unknown"
)

// Secure boot options
const (
	SecureBootRequired = "required"
	SecureBootDisabled = "disabled"
	SecureBootOptional = "optional"
)

// Machine types
const (
	PC    = "pc"
	Q35   = "q35"
	PcQ35 = "pc-q35"
)

// Firmware types
const (
	BIOS = "bios"
	EFI  = "uefi"
)

// CPU Policies
const (
	CpuPolicyDedicated = "dedicated"
	CpuPolicyShared    = "shared"
)

// CPU Thread policies
const (
	CpuThreadPolicyPrefer  = "prefer"
	CpuThreadPolicyIsolate = "isolate"
	CpuThreadPolicyRequire = "require"
)

// Bus types
const (
	ScsiBus   = "scsi"
	VirtioBus = "virtio"
	SataBus   = "sata"
	UmlBus    = "uml"
	XenBus    = "xen"
	IdeBus    = "ide"
	UsbBus    = "usb"
	LxcBus    = "lxc"
)

// Input types
const (
	Tablet    = "tablet"
	UsbTablet = "usbtablet"
)

// Video models
const (
	VideoVga    = "vga"
	VideoVirtio = "virtio"
	VideoCirrus = "cirrus"
	VideoVmVga  = "vmvga"
	VideoXen    = "xen"
	VideoQxl    = "qxl"
	VideoGop    = "gop"
	VideoNONE   = "none"
	VideoBochs  = "bochs"
)

// Vif models
const (
	// KVM/Xen/VMWare
	VifModelE1000 = "e1000"
	// KVM/VMWare
	VifModelE1000e = "e1000e"
	// KVM/Xen
	VifModelNe2kpci = "ne2k_pci"
	VifModelPcnet   = "pcnet"
	VifModelRtl8139 = "rtl8139"
	// KVM
	VifModelVmxnet3 = "vmxnet3"
	VifModelVirtio  = "virtio"
	// VMWare
	VifModelVirtualE1000   = "VirtualE1000"
	VifModelVirtualE1000e  = "VirtualE1000e"
	VifModelVirtualPcnet32 = "VirtualPCNet32"
	VifModelVirtualVmxnet  = "VirtualVmxnet"
	VifModelVirtualVmxnet3 = "VirtualVmxnet3"
	//Xen
	VifModelNetfront = "netfront"
)

// HW RNG models
const (
	HwRngModelVirtio = "virtio"
)

// Disk Formats
const (
	AMI   = "ami"
	ARI   = "ari"
	AKI   = "aki"
	VHD   = "vhd"
	VHDX  = "vhdx"
	VMDK  = "vmdk"
	RAW   = "raw"
	QCOW2 = "qcow2"
	VDI   = "vdi"
	PLOOP = "ploop"
	ISO   = "iso"
)

// Image Properties
const (
	Architecture         = "architecture"
	HypervisorType       = "hypervisor_type"
	CpuPolicy            = "hw_cpu_policy"
	CpuThreadPolicy      = "hw_cpu_thread_policy"
	CpuCores             = "hw_cpu_cores"
	CpuSockets           = "hw_cpu_sockets"
	CpuThreads           = "hw_cpu_threads"
	FirmwareType         = "hw_firmware_type"
	MachineType          = "hw_machine_type"
	CdromBus             = "hw_cdrom_bus"
	PointerModel         = "hw_pointer_model"
	VideoModel           = "hw_video_model"
	DiskBus              = "hw_disk_bus"
	VifModel             = "hw_vif_model"
	OsType               = "os_type"
	OsDistro             = "os_distro"
	OsVersion            = "os_version"
	OsSecureBoot         = "os_secure_boot"
	HwVideoRam           = "hw_video_ram"
	HwRngModel           = "hw_rng_model"
	VifMultiQueueEnabled = "hw_vif_multiqueue_enabled"
)

// Flavor ExtraSpecs
const (
	FlavorSecureBoot           = "os:secure_boot"
	FlavorCpuPolicy            = "hw:cpu_policy"
	FlavorCpuThreadPolicy      = "hw:cpu_thread_policy"
	FlavorEmulatorThreadPolicy = "hw:emulator_threads_policy"
	FlavorCpuCores             = "hw:cpu_cores"
	FlavorCpuSockets           = "hw:cpu_sockets"
	FlavorCpuThreads           = "hw:cpu_threads"
	FlavorMaxCpuCores          = "hw:max_cpu_cores"
	FlavorMaxCpuSockets        = "hw:max_cpu_sockets"
	FlavorMaxCpuThreads        = "hw:max_cpu_threads"
	FlavorVifMultiQueueEnabled = "hw:vif_multiqueue_enabled"
	FlavorHwRng                = "hw_rng:allowed"
	FlavorHwVideoRam           = "hw_video:ram_max_mb"
)

// Network types
const (
	Pod    = "pod"
	Multus = "multus"
)

// Default properties
var DefaultProperties = map[string]string{
	CpuPolicy:       CpuPolicyShared,
	CpuThreadPolicy: CpuThreadPolicyPrefer,
	FirmwareType:    BIOS,
	MachineType:     PC,
	CdromBus:        IdeBus,
	PointerModel:    UsbTablet,
	VideoModel:      VideoVirtio,
	DiskBus:         VirtioBus,
	VifModel:        VifModelVirtio,
	OsType:          Linux,
	OsSecureBoot:    SecureBootDisabled,
	HwRngModel:      HwRngModelVirtio,
}

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, vmSpec *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
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

	if vmSpec.Template == nil {
		vmSpec.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}

	r.mapFirmware(vm, vmSpec)
	r.mapResources(vm, vmSpec)
	r.mapHardwareRng(vm, vmSpec)
	r.mapInput(vm, vmSpec)
	r.mapVideo(vm, vmSpec)
	r.mapDisks(vm, persistentVolumeClaims, vmSpec)
	err = r.mapNetworks(vm, vmSpec)
	if err != nil {
		err = liberr.Wrap(
			err,
			"network mapping failed",
			"vm",
			vmRef.String())
		return
	}

	return
}

// Get list of destination VMs with mac addresses that would
// conflict with this VM, if any exist.
func (r *Builder) macConflicts(vm *model.Workload) (conflictingVMs []string, err error) {
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

	for _, vmAddresses := range vm.Addresses {
		if nics, ok := vmAddresses.([]interface{}); ok {
			for _, nic := range nics {
				if m, ok := nic.(map[string]interface{}); ok {
					if macAddress, ok := m["OS-EXT-IPS-MAC:mac_addr"]; ok {
						if conflictingVm, found := r.macConflictsMap[macAddress.(string)]; found {
							for i := range conflictingVMs {
								// ignore duplicates
								if conflictingVMs[i] == conflictingVm {
									continue
								}
							}
							conflictingVMs = append(conflictingVMs, conflictingVm)
						}
					}
				}
			}
		}
	}

	return
}

func (r *Builder) mapHardwareRng(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	allowed := false
	if flavorHwRngAllowed, ok := vm.Flavor.ExtraSpecs[FlavorHwRng]; ok {
		allowed = flavorHwRngAllowed == "true"

	}
	if allowed {
		if _, ok := vm.Image.Properties[HwRngModel]; ok {
			object.Template.Spec.Domain.Devices.Rng = &cnv.Rng{}
		}
	}
}

func (r *Builder) mapFirmware(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	var firmwareType string
	var bootloader *cnv.Bootloader
	if imageFirmwareType, ok := vm.Image.Properties[FirmwareType]; ok {
		firmwareType = imageFirmwareType.(string)
	} else {
		for _, volume := range vm.Volumes {
			if volume.Bootable == "true" {
				if volumeFirmwareType, ok := volume.VolumeImageMetadata[FirmwareType]; ok {
					firmwareType = volumeFirmwareType
				}
			}
		}
	}
	switch firmwareType {
	case EFI:
		// We disable secure boot even if it was enabled on the source because the guest OS won't
		// be able to boot without getting the NVRAM data. So we start the VM without secure boot
		// to ease the procedure users need to do in order to make the guest OS to boot.
		secureBootEnabled := false
		bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBootEnabled,
			}}
	default:
		bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	features := &cnv.Features{}
	firmware := &cnv.Firmware{}
	firmware.Bootloader = bootloader
	object.Template.Spec.Domain.Features = features
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapVideo(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	videoModel := DefaultProperties[VideoModel]
	if imageVideoModel, ok := vm.Image.Properties[VideoModel]; ok {
		videoModel = imageVideoModel.(string)
	}
	autoAttachGraphicsDevice := videoModel != VideoNONE
	object.Template.Spec.Domain.Devices.AutoattachGraphicsDevice = &autoAttachGraphicsDevice
}

func (r *Builder) mapInput(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	if _, ok := vm.Image.Properties[PointerModel]; ok {
		tablet := cnv.Input{
			Type: Tablet,
			Name: Tablet,
			Bus:  UsbBus,
		}
		object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{tablet}
	}
}

func (r *Builder) mapResources(vm *model.Workload, object *cnv.VirtualMachineSpec) {

	// KubeVirt supports Q35 or PC-Q35 machine types only.
	object.Template.Spec.Domain.Machine = &cnv.Machine{Type: Q35}
	object.Template.Spec.Domain.CPU = &cnv.CPU{}

	// Set CPU Policy
	cpuPolicy := DefaultProperties[CpuPolicy]
	if flavorCPUPolicy, ok := vm.Flavor.ExtraSpecs[FlavorCpuPolicy]; ok {
		cpuPolicy = flavorCPUPolicy
	} else if imageCPUPolicy, ok := vm.Image.Properties[CpuPolicy]; ok {
		cpuPolicy = imageCPUPolicy.(string)
	}

	if cpuPolicy == CpuPolicyDedicated {
		object.Template.Spec.Domain.CPU.DedicatedCPUPlacement = true
	}

	if flavorEmulatorThreadPolicy, ok := vm.Flavor.ExtraSpecs[FlavorEmulatorThreadPolicy]; ok {
		if flavorEmulatorThreadPolicy == CpuThreadPolicyIsolate {
			object.Template.Spec.Domain.CPU.IsolateEmulatorThread = true
		}
	}

	// Set CPU Sockets/Cores/Threads and Memory requests
	// TODO support NUMA, CPU pinning
	object.Template.Spec.Domain.CPU.Sockets = r.getCpuCount(vm, CpuSockets)
	object.Template.Spec.Domain.CPU.Cores = r.getCpuCount(vm, CpuCores)
	object.Template.Spec.Domain.CPU.Threads = r.getCpuCount(vm, CpuThreads)

	// TODO Support HugePages
	memory := resource.NewQuantity(int64(vm.Flavor.RAM)*1024*1024, resource.BinarySI)
	resourceRequests := map[core.ResourceName]resource.Quantity{}
	resourceRequests[core.ResourceMemory] = *memory

	object.Template.Spec.Domain.Resources.Requests = resourceRequests
}

func (r *Builder) getCpuCount(vm *model.Workload, imageCpuProperty string) (count uint32) {
	var flavorCpuProperty string
	switch imageCpuProperty {
	case CpuSockets:
		count = uint32(vm.Flavor.VCPUs)
		flavorCpuProperty = FlavorCpuSockets
	case CpuCores:
		count = 1
		flavorCpuProperty = FlavorCpuCores
	case CpuThreads:
		count = 1
		flavorCpuProperty = FlavorCpuThreads
	default:
		count = 0
		return
	}
	if imageCountIface, ok := vm.Image.Properties[imageCpuProperty]; ok {
		if imageCountStr, ok := imageCountIface.(string); ok {
			if imageCount, err := strconv.Atoi(imageCountStr); err == nil {
				count = uint32(imageCount)
			} else {
				r.Log.Error(err, "unable to parse image property",
					"property", imageCpuProperty, "value", imageCountStr)
			}
		}
	} else if flavorCountStr, ok := vm.Flavor.ExtraSpecs[flavorCpuProperty]; ok {
		if flavorCount, err := strconv.Atoi(flavorCountStr); err == nil {
			count = uint32(flavorCount)
		} else {
			r.Log.Error(err, "unable to parse flavor extra spec",
				"extraSpec", flavorCpuProperty, "value", flavorCountStr)
		}
	}
	return
}

func (r *Builder) mapDisks(vm *model.Workload, persistentVolumeClaims []core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	// The disk bus is common for all the VM disks and it's configured in the image properties.
	bus := DefaultProperties[DiskBus]
	if imageDiskBus, ok := vm.Image.Properties[DiskBus]; ok {
		bus = imageDiskBus.(string)
	} else {
		for _, volume := range vm.Volumes {
			if volume.Bootable == "true" {
				if volumeDiskBus, ok := volume.VolumeImageMetadata[DiskBus]; ok {
					bus = volumeDiskBus
				}
			}
		}
	}
	// Only q35 machine type is supported in Kubevirt so we need to map
	// openstack bus types to supported ones
	switch bus {
	case IdeBus:
		bus = SataBus
	case ScsiBus:
		bus = ScsiBus
	default:
		bus = VirtioBus
	}

	for _, pvc := range persistentVolumeClaims {
		image, err := r.getImageFromPVC(&pvc)
		if err != nil {
			r.Log.Error(err, "image not found in inventory", "imageID", pvc.Name)
			return
		}

		cnvVolumeName := fmt.Sprintf("vol-%v", pvc.Annotations[AnnImportDiskId])
		cnvVolume := cnv.Volume{
			Name: cnvVolumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}
		var disk cnv.Disk
		switch image.DiskFormat {
		case ISO:
			// Map CDRom
			if CDROMBus, ok := vm.Image.Properties[CdromBus]; ok {
				bus = CDROMBus.(string)
			}
			disk = cnv.Disk{
				Name: cnvVolumeName,
				DiskDevice: cnv.DiskDevice{
					CDRom: &cnv.CDRomTarget{
						Bus: cnv.DiskBus(bus),
					},
				},
			}
		case QCOW2, RAW:
			disk = cnv.Disk{
				Name: cnvVolumeName,
				DiskDevice: cnv.DiskDevice{
					Disk: &cnv.DiskTarget{
						Bus: cnv.DiskBus(bus),
					},
				},
			}
		default:
			r.Log.Info("image disk format not supported", "format", image.DiskFormat)
		}
		kVolumes = append(kVolumes, cnvVolume)
		kDisks = append(kDisks, disk)
	}

	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

func (r *Builder) mapNetworks(vm *model.Workload, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	for vmNetworkName, vmAddresses := range vm.Addresses {
		if nics, ok := vmAddresses.([]interface{}); ok {
			for _, nic := range nics {
				networkName := fmt.Sprintf("net-%v", numNetworks)
				kNetwork := cnv.Network{
					Name: networkName,
				}
				kInterface := cnv.Interface{
					Name: networkName,
				}
				var interfaceModel string
				vifModel := DefaultProperties[VifModel]
				if imageVIFModel, ok := vm.Image.Properties[VifModel]; ok {
					vifModel = imageVIFModel.(string)
				}
				switch vifModel {
				case VifModelVirtualE1000:
					interfaceModel = VifModelE1000
				case VifModelVirtualE1000e:
					interfaceModel = VifModelE1000e
				case VifModelVirtualPcnet32:
					interfaceModel = VifModelPcnet
				case VifModelE1000, VifModelE1000e, VifModelNe2kpci, VifModelPcnet, VifModelRtl8139, VifModelVirtio:
					interfaceModel = vifModel
				default:
					interfaceModel = DefaultProperties[VifModel]
				}
				kInterface.Model = interfaceModel
				if m := nic.(map[string]interface{}); ok {
					if macAddress, ok := m["OS-EXT-IPS-MAC:mac_addr"]; ok {
						kInterface.MacAddress = macAddress.(string)
					}
					if ipType, ok := m["OS-EXT-IPS:type"]; ok {
						if ipType.(string) == "floating" {
							continue
						}
					}
				}

				var vmNetworkID string
				for _, vmNetwork := range vm.Networks {
					if vmNetwork.Name == vmNetworkName {
						vmNetworkID = vmNetwork.ID
						break
					}
				}
				var networkPair *api.NetworkPair
				networkMaps := r.Context.Map.Network.Spec.Map
				found := false
				for i := range networkMaps {
					networkPair = &networkMaps[i]
					if networkPair.Source.ID == vmNetworkID {
						found = true
						break
					}
				}
				if !found {
					err = liberr.New("no network map for vm network", "network", vmNetworkID)
					return
				}
				switch networkPair.Destination.Type {
				case Pod:
					kNetwork.Pod = &cnv.PodNetwork{}
					kInterface.Masquerade = &cnv.InterfaceMasquerade{}
				case Multus:
					kNetwork.Multus = &cnv.MultusNetwork{
						NetworkName: path.Join(
							networkPair.Destination.Namespace,
							networkPair.Destination.Name),
					}
					kInterface.Bridge = &cnv.InterfaceBridge{}
				}
				kNetworks = append(kNetworks, kNetwork)
				kInterfaces = append(kInterfaces, kInterface)
				numNetworks++
			}
		}
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces

	var vifMultiQueueEnabled *bool
	if imageVifMultiQueueEnabled, ok := vm.Image.Properties[VifMultiQueueEnabled]; ok {
		if enabledStr, ok := imageVifMultiQueueEnabled.(string); ok {
			if enabled, err := strconv.ParseBool(enabledStr); err == nil && enabled {
				vifMultiQueueEnabled = &enabled
			} else if err != nil {
				r.Log.Error(err, "unable to parse image property",
					"property", VifMultiQueueEnabled, "value", enabledStr)
			}
		}
	} else if flavorVifMultiQueueEnabled, ok := vm.Flavor.ExtraSpecs[FlavorVifMultiQueueEnabled]; ok {
		if enabled, err := strconv.ParseBool(flavorVifMultiQueueEnabled); err == nil && enabled {
			vifMultiQueueEnabled = &enabled
		} else if err != nil {
			r.Log.Error(err, "unable to parse flavor extra spec",
				"extraSpec", FlavorVifMultiQueueEnabled, "value", flavorVifMultiQueueEnabled)
		}
	}
	if vifMultiQueueEnabled != nil {
		object.Template.Spec.Domain.Devices.NetworkInterfaceMultiQueue = vifMultiQueueEnabled
	}

	return
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	workload := &model.Workload{}
	err = r.Source.Inventory.Find(workload, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
	}

	taskMap := map[string]int64{}
	imageID := workload.ImageID

	if imageID != "" {
		taskName := getVmSnapshotName(r.Context, workload.ID)
		taskTotal := int64(0)
		taskTotal = workload.Image.SizeBytes / 1024 / 1024
		taskMap[taskName] = taskTotal
	}

	for _, volume := range workload.Volumes {
		taskName := getImageFromVolumeName(r.Context, workload.ID, volume.ID)
		taskTotal := int64(volume.Size * 1024)
		taskMap[taskName] = taskTotal
	}

	for taskName, taskTotal := range taskMap {
		r.Log.Info("creating task", "taskName", taskName, "taskTotal", taskTotal)
		task := &plan.Task{
			Name: taskName,
			Progress: libitr.Progress{
				Total: taskTotal,
			},
			Annotations: map[string]string{
				"unit": "MB",
			},
		}
		r.Log.Info("adding task to the plan", "task", task.Name)
		tasks = append(tasks, task)
	}

	return
}

// Create DataVolume certificate configmap.
func (r *Builder) ConfigMap(_ ref.Ref, in *core.Secret, object *core.ConfigMap) (err error) {
	return
}

func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error) {
	return nil, nil
}

// Build tasks.
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	os := ""
	distro := ""
	version := ""

	if osDistro, ok := vm.Image.Properties[OsDistro]; ok {
		distro = osDistro.(string)
	}

	if osVersion, ok := vm.Image.Properties[OsVersion]; ok {
		version = osVersion.(string)
	}

	switch distro {
	case ArchLinux, Debian, Gentoo, Mandrake, Mandriva, MES:
		os = UnknownOS
	case FreeBSD, OpenBSD, NetBSD:
		os = UnknownOS
	case RHEL, CentOS, Fedora, Ubuntu, OpenSUSE, Windows:
		os = distro
	case SLED:
		os = OpenSUSE
	case MSDOS:
		os = Windows
	default:
		os = UnknownOS
	}

	if os != UnknownOS && version != "" {
		os = fmt.Sprintf("%s%s", os, version)
		if distro == CentOS && len(version) >= 1 && (version[:1] == "8" || version[:1] == "9") {
			os = fmt.Sprintf("%s-stream%s", distro, version)
		} else if os == Windows {
			os = DefaultWindows
			if strings.Contains(version, "2k12") || strings.Contains(version, "2012") {
				os = fmt.Sprintf("%s2k12", os)
			} else if strings.Contains(version, "2k16") || strings.Contains(version, "2016") {
				os = fmt.Sprintf("%s2k16", os)
			} else if strings.Contains(version, "2k19") || strings.Contains(version, "2019") {
				os = fmt.Sprintf("%s2k19", os)
			} else if strings.Contains(version, "2k22") || strings.Contains(version, "2022") {
				os = fmt.Sprintf("%s2k22", os)
			} else if len(version) >= 2 && version[:2] == "11" {
				os = fmt.Sprintf("%s%s", os, version)
			}
		}
	}

	var flavor string

	ram := vm.Flavor.RAM
	switch {
	case ram > 8192:
		flavor = TemplateFlavorLarge
	case ram > 4096 && ram < 8192:
		flavor = TemplateFlavorMedium
	case ram > 2048 && ram < 4096:
		flavor = TemplateFlavorSmall
	default:
		flavor = TemplateFlavorTiny
	}

	workload := TemplateWorkloadServer

	if _, ok := vm.Image.Properties[PointerModel]; ok {
		workload = TemplateWorkloadDesktop
	}

	if flavorEmulatorThreadPolicy, ok := vm.Flavor.ExtraSpecs[FlavorEmulatorThreadPolicy]; ok {
		if flavorEmulatorThreadPolicy == CpuThreadPolicyIsolate {
			workload = TemplateWorkloadHighPerformance
		}
	}

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[fmt.Sprintf(TemplateWorkloadLabel, workload)] = "true"
	labels[fmt.Sprintf(TemplateFlavorLabel, flavor)] = "true"

	return
}

// Return a stable identifier for a DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}

// Return a stable identifier for a PersistentDataVolume
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

// Build credential secret.
func (r *Builder) Secret(_ ref.Ref, in, secret *core.Secret) (err error) {
	// no-op, we just need to clone the provider secret so there's no action to be made here
	return
}

func (r *Builder) PodEnvironment(_ ref.Ref, _ *core.Secret) (env []core.EnvVar, err error) {
	return
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
	return true
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcNames []string, err error) {
	workload := &model.Workload{}
	err = r.Source.Inventory.Find(workload, vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	images, err := r.getImagesFromVolumes(workload)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if workload.ImageID != "" {
		var image model.Image
		image, err = r.getVMSnapshotImage(workload)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		images = append(images, image)
	}

	for _, image := range images {

		if image.Status != string(ImageStatusActive) {
			r.Log.Info("the image is not ready yet", "image", image.Name)
			continue
		}

		originalVolumeDiskId := image.Name
		if imageProperty, ok := image.Properties[forkliftPropertyOriginalVolumeID]; ok {
			originalVolumeDiskId = imageProperty.(string)
		}

		_, err = r.getVolumePopulator(image.Name)
		if err != nil {
			if !k8serr.IsNotFound(err) {
				err = liberr.Wrap(err)
				return
			}
		}

		var populatorName string
		populatorName, err = r.createVolumePopulatorCR(image, secretName, vmRef.ID)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		storageClassName := r.Context.Map.Storage.Spec.Map[0].Destination.StorageClass
		volumeType := r.getVolumeType(workload, originalVolumeDiskId)
		if volumeType != "" {
			storageClassName, err = r.getStorageClassName(workload, volumeType)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
		}

		var pvc *core.PersistentVolumeClaim
		pvc, err = r.persistentVolumeClaimWithSourceRef(image, storageClassName, populatorName, annotations)
		if err != nil {
			if !k8serr.IsAlreadyExists(err) {
				err = liberr.Wrap(err, "couldn't build the PVC",
					"image", image.Name, "storageClassName", storageClassName, "populatorName", populatorName)
				return
			}
			err = nil
			continue
		}
		pvcNames = append(pvcNames, pvc.Name)
	}
	return
}

func (r *Builder) getVMSnapshotImage(workload *model.Workload) (image model.Image, err error) {
	image = model.Image{}
	imageName := getVmSnapshotName(r.Context, workload.ID)
	err = r.Source.Inventory.Find(&image, ref.Ref{Name: imageName})
	if err != nil {
		if errors.As(err, &model.NotFoundError{}) {
			err = nil
			r.Log.Info("the vm snapshot image has not been created yet", "imageName", imageName)
			return
		}
		r.Log.Error(err, "error retrieving the vm snapshot image information", "imageName", imageName)
		return
	}
	r.Log.Info("appending vm snapshot image", "imageName", imageName)
	return
}

func (r *Builder) getImagesFromVolumes(workload *model.Workload) (images []model.Image, err error) {
	images = []model.Image{}
	for _, volume := range workload.Volumes {
		image := model.Image{}
		imageName := getImageFromVolumeName(r.Context, workload.ID, volume.ID)
		err = r.Source.Inventory.Find(&image, ref.Ref{Name: imageName})
		if err != nil {
			if errors.As(err, &model.NotFoundError{}) {
				err = nil
				r.Log.Info("the image from volume has not been created yet", "imageName", imageName)
				continue
			}
			r.Log.Error(err, "error retrieving the image from volume information", "imageName", imageName)
			return
		}
		if _, ok := image.Properties[forkliftPropertyOriginalVolumeID]; !ok {
			r.Log.Info("the image properties have not been updated yet", "image", image.Name)
			continue
		}
		r.Log.Info("appending image from volume", "imageName", imageName)
		images = append(images, image)
	}
	return
}

func (r *Builder) createVolumePopulatorCR(image model.Image, secretName, vmId string) (name string, err error) {
	populatorCR := &api.OpenstackVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			Name:      image.Name,
			Namespace: r.Plan.Spec.TargetNamespace,
			Labels:    map[string]string{"vmID": vmId, "migration": getMigrationID(r.Context)},
		},
		Spec: api.OpenstackVolumePopulatorSpec{
			IdentityURL:     r.Source.Provider.Spec.URL,
			SecretName:      secretName,
			ImageID:         image.ID,
			TransferNetwork: r.Plan.Spec.TransferNetwork,
		},
	}
	err = r.Context.Client.Create(context.TODO(), populatorCR, &client.CreateOptions{})
	if err != nil {
		if !k8serr.IsAlreadyExists(err) {
			err = liberr.Wrap(err)
			return
		} else {
			err = nil
		}
	}
	name = populatorCR.Name
	return
}

func (r *Builder) getVolumeType(workload *model.Workload, volumeID string) (volumeType string) {
	for _, volume := range workload.Volumes {
		if volume.ID == volumeID {
			volumeType = volume.VolumeType
			return
		}
	}
	return
}

func (r *Builder) getStorageClassName(workload *model.Workload, volumeTypeName string) (storageClassName string, err error) {
	var volumeTypeID string
	for _, volumeType := range workload.VolumeTypes {
		if volumeTypeName == volumeType.Name {
			volumeTypeID = volumeType.ID
		}
	}
	if volumeTypeID == "" {
		err = liberr.New("volume type not found", "volumeType", volumeTypeName)
		return
	}
	for _, storageMap := range r.Context.Map.Storage.Spec.Map {
		if storageMap.Source.ID == volumeTypeID {
			storageClassName = storageMap.Destination.StorageClass
		}
	}
	if storageClassName == "" {
		err = liberr.New("no storage class map found for volume type", "volumeTypeID", volumeTypeID)
		return
	}
	return
}

// Using CDI logic to set the Volume mode and Access mode of the PVC - https://github.com/kubevirt/containerized-data-importer/blob/v1.56.0/pkg/controller/datavolume/util.go#L154
func (r *Builder) getVolumeAndAccessMode(storageClassName string) ([]core.PersistentVolumeAccessMode, *core.PersistentVolumeMode, error) {
	filesystemMode := core.PersistentVolumeFilesystem
	storageProfile := &cdi.StorageProfile{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, storageProfile)
	if err != nil {
		return nil, nil, liberr.Wrap(err, "cannot get storage profile", "storageClassName", storageClassName)
	}

	if len(storageProfile.Status.ClaimPropertySets) > 0 &&
		len(storageProfile.Status.ClaimPropertySets[0].AccessModes) > 0 {
		accessModes := storageProfile.Status.ClaimPropertySets[0].AccessModes
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		if volumeMode == nil {
			// volumeMode is an optional API parameter. Filesystem is the default mode used when volumeMode parameter is omitted.
			volumeMode = &filesystemMode
		}
		return accessModes, volumeMode, nil
	}

	// no accessMode configured on storageProfile
	return nil, nil, liberr.New("no accessMode defined on StorageProfile for StorageClass", "storageClassName", storageClassName)

}

// Get the OpenstackVolumePopulator CustomResource based on the image name.
func (r *Builder) getVolumePopulator(name string) (populatorCr api.OpenstackVolumePopulator, err error) {
	populatorCr = api.OpenstackVolumePopulator{}
	err = r.Destination.Client.Get(context.TODO(), client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: name}, &populatorCr)
	return
}

func (r *Builder) persistentVolumeClaimWithSourceRef(image model.Image, storageClassName string,
	populatorName string, annotations map[string]string) (pvc *core.PersistentVolumeClaim, err error) {

	apiGroup := "forklift.konveyor.io"
	virtualSize := image.VirtualSize
	// virtual_size may not always be available
	if virtualSize == 0 {
		virtualSize = image.SizeBytes
	}

	var accessModes []core.PersistentVolumeAccessMode
	var volumeMode *core.PersistentVolumeMode
	accessModes, volumeMode, err = r.getVolumeAndAccessMode(storageClassName)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if *volumeMode == core.PersistentVolumeFilesystem {
		virtualSize = utils.CalculateSpaceWithOverhead(virtualSize, 0.1)
	}

	// The image might be a VM Snapshot Image and has no volume associated to it
	if originalVolumeDiskId, ok := image.Properties["forklift_original_volume_id"]; ok {
		annotations[AnnImportDiskId] = originalVolumeDiskId.(string)
		r.Log.Info("the image comes from a volume", "volumeID", originalVolumeDiskId)
	} else {
		annotations[AnnImportDiskId] = image.ID
		r.Log.Info("the image comes from a vm snapshot", "imageID", image.ID)
	}

	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:        image.ID,
			Namespace:   r.Plan.Spec.TargetNamespace,
			Annotations: annotations,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: core.ResourceRequirements{
				Requests: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: *resource.NewQuantity(virtualSize, resource.BinarySI)},
			},
			StorageClassName: &storageClassName,
			VolumeMode:       volumeMode,
			DataSourceRef: &core.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     api.OpenstackVolumePopulatorKind,
				Name:     populatorName,
			},
		},
	}

	err = r.Client.Create(context.TODO(), pvc, &client.CreateOptions{})
	return
}

func (r *Builder) PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	image, err := r.getImageFromPVC(persistentVolumeClaim)
	if err != nil {
		return
	}
	populatorCr, err := r.getVolumePopulator(image.Name)
	if err != nil {
		return
	}
	transferredBytes, err = strconv.ParseInt(populatorCr.Status.Transferred, 10, 64)
	if err != nil {
		transferredBytes = 0
		err = nil
		return
	}
	return
}

// Get the Openstack image from the inventory based on the PVC.
func (r *Builder) getImageFromPVC(pvc *core.PersistentVolumeClaim) (image *model.Image, err error) {
	image = &model.Image{}
	err = r.Source.Inventory.Find(image, ref.Ref{ID: pvc.Name})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []core.PersistentVolumeClaim) (err error) {
	workload := &model.Workload{}
	err = r.Source.Inventory.Find(workload, vmRef)
	if err != nil {
		return
	}
	var images []*model.Image
	for _, volume := range workload.Volumes {
		lookupName := getImageFromVolumeName(r.Context, vmRef.ID, volume.ID)
		image, err := r.getImageByName(lookupName)
		if err != nil {
			continue
		}
		images = append(images, image)
	}
	if len(images) != len(pvcs) {
		// To be sure we have every disk based on what already migrated and what's not.
		// e.g when initializing the plan and the PVC has not been created yet (but the populator CR is) or when the disks that are attached to the source VM change.
		for _, pvc := range pvcs {
			image, err := r.getImageFromPVC(&pvc)
			if err != nil {
				continue
			}
			images = append(images, image)
		}
	}
	migrationID := string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)
	for _, image := range images {
		populatorCr, err := r.getVolumePopulator(image.Name)
		if err != nil {
			continue
		}
		err = r.setPopulatorLabels(populatorCr, vmRef.ID, migrationID)
		if err != nil {
			r.Log.Error(err, "Couldn't update the Populator Custom Resource labels.",
				"vmRef", vmRef, "migration", migrationID, "OpenStackVolumePopulator", populatorCr.Name)
			continue
		}
	}
	return
}

// Get the Openstack image from the inventory based on the name.
func (r *Builder) getImageByName(name string) (image *model.Image, err error) {
	image = &model.Image{}
	err = r.Source.Inventory.Find(image, ref.Ref{Name: name})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}
func (r *Builder) setPopulatorLabels(populatorCr api.OpenstackVolumePopulator, vmId, migrationId string) (err error) {
	populatorCrCopy := populatorCr.DeepCopy()
	if populatorCr.Labels == nil {
		populatorCr.Labels = make(map[string]string)
	}
	populatorCr.Labels["vmID"] = vmId
	populatorCr.Labels["migration"] = migrationId
	patch := client.MergeFrom(populatorCrCopy)
	err = r.Destination.Client.Patch(context.TODO(), &populatorCr, patch)
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	image, err := r.getImageFromPVC(pvc)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	taskName = image.Name
	return
}
