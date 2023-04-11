package openstack

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/volumeactions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/client-go/api/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
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
	VifModelE1000   = "e1000"
	VifModelE1000e  = "e1000e"
	VifModelNe2kpci = "ne2k_pci"
	VifModelPcnet   = "pcnet"
	VifModelRtl8139 = "rtl8139"
	VifModelVirtio  = "virtio"
	VifModelVmxnet3 = "vmxnet3"
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
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
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

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}

	r.mapFirmware(vm, object)
	r.mapResources(vm, object)
	r.mapHardwareRng(vm, object)
	r.mapInput(vm, object)
	r.mapVideo(vm, object)
	r.mapDisks(vm, persistentVolumeClaims, object)
	err = r.mapNetworks(vm, object)
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
	var cpuSockets, cpuCores, cpuThreads int

	cpuSockets = vm.Flavor.VCPUs
	cpuCores = 1
	cpuThreads = 1

	if flavorCPUSockets, ok := vm.Flavor.ExtraSpecs[FlavorCpuSockets]; ok {
		if sockets, err := strconv.Atoi(flavorCPUSockets); err == nil {
			cpuSockets = sockets
		}
	} else if imageCPUSockets, ok := vm.Image.Properties[CpuSockets]; ok {
		if sockets, ok := imageCPUSockets.(int); ok {
			cpuSockets = sockets
		}
	}
	if flavorCPUCores, ok := vm.Flavor.ExtraSpecs[FlavorCpuCores]; ok {
		if cores, err := strconv.Atoi(flavorCPUCores); err == nil {
			cpuCores = cores
		}
	} else if imageCPUCores, ok := vm.Image.Properties[CpuCores]; ok {
		if cores, ok := imageCPUCores.(int); ok {
			cpuCores = cores
		}
	}
	if flavorCPUThreads, ok := vm.Flavor.ExtraSpecs[FlavorCpuThreads]; ok {
		if threads, err := strconv.Atoi(flavorCPUThreads); err == nil {
			cpuThreads = threads
		}
	} else if imageCPUThreads, ok := vm.Image.Properties[CpuThreads]; ok {
		if threads, ok := imageCPUThreads.(int); ok {
			cpuThreads = threads
		}
	}

	resourceRequests := map[core.ResourceName]resource.Quantity{}

	object.Template.Spec.Domain.CPU.Sockets = uint32(cpuSockets)
	object.Template.Spec.Domain.CPU.Cores = uint32(cpuCores)
	object.Template.Spec.Domain.CPU.Threads = uint32(cpuThreads)

	// TODO Support HugePages
	memory := resource.NewQuantity(int64(vm.Flavor.RAM)*1024*1024, resource.BinarySI)
	resourceRequests[core.ResourceMemory] = *memory

	object.Template.Spec.Domain.Resources.Requests = resourceRequests
}

func (r *Builder) mapDisks(vm *model.Workload, persistentVolumeClaims []core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	pvcMap := make(map[string]*core.PersistentVolumeClaim)

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

	for i := range persistentVolumeClaims {
		pvc := &persistentVolumeClaims[i]
		pvcMap[pvc.Annotations[AnnImportDiskId]] = pvc
	}
	for i, av := range vm.Volumes {
		image := &model.Image{}
		err := r.Source.Inventory.Find(image, ref.Ref{Name: fmt.Sprintf("%s-%s", r.Migration.Name, av.ID)})
		if err != nil {
			return
		}
		pvc := pvcMap[av.ID]
		volumeName := fmt.Sprintf("vol-%v", i)
		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
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
				Name: volumeName,
				DiskDevice: cnv.DiskDevice{
					CDRom: &cnv.CDRomTarget{
						Bus: bus,
					},
				},
			}
		case QCOW2, RAW:
			disk = cnv.Disk{
				Name: volumeName,
				DiskDevice: cnv.DiskDevice{
					Disk: &cnv.DiskTarget{
						Bus: bus,
					},
				},
			}
		default:
			r.Log.Info("image disk format not supported", "format", image.DiskFormat)
		}
		kVolumes = append(kVolumes, volume)
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
				interfaceModel := DefaultProperties[VifModel]
				if imageVIFModel, ok := vm.Image.Properties[VifModel]; ok {
					interfaceModel = imageVIFModel.(string)
				}
				kInterface.Model = interfaceModel
				if m := nic.(map[string]interface{}); ok {
					if macAddress, ok := m["OS-EXT-IPS-MAC:mac_addr"]; ok {
						kInterface.MacAddress = macAddress.(string)
					}
				}

				var vmNetworkID string
				for _, vmNetwork := range vm.Networks {
					if vmNetwork.Name == vmNetworkName {
						vmNetworkID = vmNetwork.ID
						break
					}
				}
				var networkPair *v1beta1.NetworkPair
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
	if flavorVifMultiQueueEnabled, ok := vm.Flavor.ExtraSpecs[FlavorVifMultiQueueEnabled]; ok {
		if enabled, err := strconv.ParseBool(flavorVifMultiQueueEnabled); err == nil && enabled {
			vifMultiQueueEnabled = &enabled
		}
	} else if imageVifMultiQueueEnabled, ok := vm.Image.Properties[VifMultiQueueEnabled]; ok {
		if enabled, ok := imageVifMultiQueueEnabled.(bool); ok {
			vifMultiQueueEnabled = &enabled
		}
	}
	if vifMultiQueueEnabled != nil {
		object.Template.Spec.Domain.Devices.NetworkInterfaceMultiQueue = vifMultiQueueEnabled
	}

	return
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
	}

	for _, va := range vm.Volumes {
		gb := int64(va.Size)
		list = append(
			list,
			&plan.Task{
				Name: fmt.Sprintf("%s-%s", r.Migration.Name, va.ID),
				Progress: libitr.Progress{
					Total: gb * 1024,
				},
				Annotations: map[string]string{
					"unit": "MB",
				},
			})
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

func (r *Builder) PersistentVolumeClaimWithSourceRef(da interface{}, storageName *string, populatorName string, accessModes []core.PersistentVolumeAccessMode, volumeMode *core.PersistentVolumeMode) *core.PersistentVolumeClaim {
	image := da.(*model.Image)
	apiGroup := "forklift.konveyor.io"
	virtualSize := image.VirtualSize
	// virtual_size may not always be available
	if virtualSize == 0 {
		virtualSize = image.SizeBytes
	}
	if *volumeMode == core.PersistentVolumeFilesystem {
		virtualSize = int64(float64(virtualSize) * 1.1)
	}
	return &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:      image.ID,
			Namespace: r.Plan.Spec.TargetNamespace,
			Annotations: map[string]string{
				AnnImportDiskId: image.Name[len(r.Migration.Name)+1:],
			},
			Labels: map[string]string{"migration": r.Migration.Name},
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: core.ResourceRequirements{
				Requests: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: *resource.NewQuantity(virtualSize, resource.BinarySI)},
			},
			StorageClassName: storageName,
			VolumeMode:       volumeMode,
			DataSourceRef: &core.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     v1beta1.OpenstackVolumePopulatorKind,
				Name:     populatorName,
			},
		},
	}
}

func (r *Builder) BeforeTransferHook(c planbase.Client, vmRef ref.Ref) (ready bool, err error) {
	// TODO:
	// 1. Dedup
	// 2. Improve concurrency, as soon as the image is ready we can create the PVC, no need to wait
	// for everything to finish
	client, ok := c.(*Client)
	if !ok {
		return false, nil
	}

	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return true, err
	}

	var snaplist []snapshots.Snapshot
	for _, av := range vm.Volumes {
		imageName := fmt.Sprintf("%s-%s", r.Migration.Name, av.ID)
		pager := snapshots.List(client.blockStorageService, snapshots.ListOpts{
			Name:  imageName,
			Limit: 1,
		})
		pages, err := pager.AllPages()
		if err != nil {
			return true, err
		}
		isEmpty, err := pages.IsEmpty()
		if err != nil {
			return true, err
		}
		if !isEmpty {
			snaps, err := snapshots.ExtractSnapshots(pages)
			if err != nil {
				return true, err
			}

			snaplist = append(snaplist, snaps...)
			continue
		}

		snapshot, err := snapshots.Create(client.blockStorageService, snapshots.CreateOpts{
			Name:        imageName,
			VolumeID:    av.ID,
			Force:       true,
			Description: imageName,
		}).Extract()
		if err != nil {
			err = liberr.Wrap(
				err,
				"Failed to create snapshot.",
				"volume",
				av.ID)
			return true, err
		}

		snaplist = append(snaplist, *snapshot)
	}

	for _, snap := range snaplist {
		snapshot, err := snapshots.Get(client.blockStorageService, snap.ID).Extract()
		if err != nil {
			return true, err
		}
		if snapshot.Status != "available" {
			r.Log.Info("Snapshot not ready yet, recheking...", "snapshot", snap.Name)
			return false, nil
		}
	}

	var vollist []volumes.Volume
	for _, snap := range snaplist {
		imageName := fmt.Sprintf("%s-%s", r.Migration.Name, snap.VolumeID)
		pager := volumes.List(client.blockStorageService, volumes.ListOpts{
			Name:  imageName,
			Limit: 1,
		})
		pages, err := pager.AllPages()
		if err != nil {
			return true, err
		}
		isEmpty, err := pages.IsEmpty()
		if err != nil {
			return true, err
		}
		if !isEmpty {
			vols, err := volumes.ExtractVolumes(pages)
			if err != nil {
				return true, err
			}
			vollist = append(vollist, vols...)

			continue
		}
		volume, err := volumes.Create(client.blockStorageService, volumes.CreateOpts{
			Name:        imageName,
			SnapshotID:  snap.ID,
			Size:        snap.Size,
			Description: imageName,
		}).Extract()
		if err != nil {
			err = liberr.Wrap(
				err,
				"Failed to create snapshot.",
				"volume",
				volume.ID)
			return true, err
		}
		vollist = append(vollist, *volume)
	}

	for _, vol := range vollist {
		volume, err := volumes.Get(client.blockStorageService, vol.ID).Extract()
		if err != nil {
			return true, err
		}

		if volume.Status != "available" && volume.Status != "uploading" {
			r.Log.Info("Volume not ready yet, recheking...", "volume", vol.Name)
			return false, nil
		}
	}

	var imagelist []string

	for _, vol := range vollist {
		pager := images.List(client.imageService, images.ListOpts{
			Name:  vol.Description,
			Limit: 1,
		})
		pages, err := pager.AllPages()
		if err != nil {
			return true, err
		}
		isEmpty, err := pages.IsEmpty()
		if err != nil {
			return true, err
		}
		if !isEmpty {
			imgs, err := images.ExtractImages(pages)
			if err != nil {
				return true, err
			}
			for _, i := range imgs {
				imagelist = append(imagelist, i.ID)
			}
			r.Log.Info("Image already exists", "id", imagelist)
			continue
		}

		image, err := volumeactions.UploadImage(client.blockStorageService, vol.ID, volumeactions.UploadImageOpts{
			ImageName:  vol.Description,
			DiskFormat: "raw",
		}).Extract()
		if err != nil {
			err = liberr.Wrap(
				err,
				"Failed to create image.",
				"image",
				image.ImageID)
			return false, err
		}

		imagelist = append(imagelist, image.ImageID)
	}

	for _, imageID := range imagelist {
		img, err := images.Get(client.imageService, imageID).Extract()
		if err != nil {
			return true, err
		}

		// TODO also check for "saving" and "error"
		if img.Status != images.ImageStatusActive {
			r.Log.Info("Image not ready yet, recheking...", "image", img)
			return false, nil
		} else if img.Status == images.ImageStatusActive {
			// TODO figure out a better way, since when the image in the inventory may be out of sync
			// with openstack, and be ready in openstack, but not in the inventory
			if !r.imageReady(img.Name) {
				r.Log.Info("Image not ready yet in inventory, recheking...", "image", img.Name)
				return false, nil
			} else {
				r.Log.Info("Image is ready, cleaning up...", "image", img.Name)
				r.cleanup(c, img.Name)
			}
		}
	}

	return true, nil
}

func (r *Builder) imageReady(imageName string) bool {
	image := &model.Image{}
	err := r.Source.Inventory.Find(image, ref.Ref{Name: imageName})
	if err == nil {
		r.Log.Info("Image status in inventory", "image", image.Status)
		return image.Status == "active"
	}
	return false
}

func (r *Builder) cleanup(c planbase.Client, imageName string) {
	client, ok := c.(*Client)
	if !ok {
		r.Log.Info("Couldn't cast client (should never happen)")
		return
	}

	volume := &model.Volume{}
	err := r.Source.Inventory.Find(volume, ref.Ref{Name: imageName})
	if err != nil {
		r.Log.Info("couldn't find volume for deletion, skipping...", "name", imageName)
	} else {
		err = volumes.Delete(client.blockStorageService, volume.ID, volumes.DeleteOpts{Cascade: true}).ExtractErr()
		if err != nil {
			r.Log.Error(err, "error removing volume", "name", imageName)
		}
	}

	snapshot := &model.Snapshot{}
	err = r.Source.Inventory.Find(snapshot, ref.Ref{Name: imageName})
	if err != nil {
		r.Log.Info("couldn't find snapshot for deletion, skipping...", "name", imageName)
	} else {
		err = snapshots.Delete(client.blockStorageService, snapshot.ID).ExtractErr()
		if err != nil {
			r.Log.Error(err, "error removing snapshot", "name", imageName)
		}
	}
}
