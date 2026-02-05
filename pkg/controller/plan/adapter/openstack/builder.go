package openstack

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	utils "github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Openstack builder.
type Builder struct {
	*plancontext.Context
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
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// Default properties
var DefaultProperties = map[string]string{
	CpuPolicy:       CpuPolicyShared,
	CpuThreadPolicy: CpuThreadPolicyPrefer,
	FirmwareType:    BIOS,
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
func (r *Builder) VirtualMachine(vmRef ref.Ref, vmSpec *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) (err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	if vmSpec.Template == nil {
		vmSpec.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}

	r.mapFirmware(vm, vmSpec)
	r.mapResources(vm, vmSpec, usesInstanceType)
	r.mapHardwareRng(vm, vmSpec)
	r.mapInput(vm, vmSpec)
	r.mapVideo(vm, vmSpec)
	r.mapDisks(vm, persistentVolumeClaims, vmSpec)
	err = r.mapNetworks(vm, vmSpec)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
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
	firmware := &cnv.Firmware{}
	firmware.Bootloader = bootloader
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

func (r *Builder) mapResources(vm *model.Workload, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
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
	object.Template.Spec.Domain.Memory = &cnv.Memory{Guest: memory}
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

func (r *Builder) mapDisks(vm *model.Workload, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
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

	var bootOrderSet bool
	var imagePVC *core.PersistentVolumeClaim
	for _, pvc := range persistentVolumeClaims {
		// Handle loopvar https://go.dev/wiki/LoopvarExperiment
		pvc := pvc

		var bootOrder *uint
		image, err := r.getImageFromPVC(pvc)
		if err != nil {
			r.Log.Error(err, "image not found in inventory", "imageID", pvc.Labels["imageID"])
			return
		}

		if imageID, ok := image.Properties[forkliftPropertyOriginalImageID]; ok && imageID != "" {
			if imageID.(string) == vm.ImageID {
				imagePVC = pvc
				r.Log.Info("Image PVC found", "pvc", pvc.Name, "image", imagePVC.Annotations[planbase.AnnDiskSource])
			}
		} else if volumeID, ok := image.Properties[forkliftPropertyOriginalVolumeID]; ok && volumeID != "" {
			// Image is volume based, check if it's bootable
			volume := &model.Volume{}
			err = r.Source.Inventory.Get(volume, volumeID.(string))
			if err != nil {
				r.Log.Error(err, "Failed to get volume from inventory", "volumeID", volumeID)
				return
			}
			if bootable, err := strconv.ParseBool(volume.Bootable); err == nil && bootable {
				r.Log.Info("bootable volume found", "volumeID", volumeID)
				bootOrder = ptr.To[uint](1)
				bootOrderSet = true
			}
		}

		cnvVolumeName := fmt.Sprintf("vol-%s", pvc.Annotations[planbase.AnnDiskSource])
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
				Name:      cnvVolumeName,
				BootOrder: bootOrder,
				DiskDevice: cnv.DiskDevice{
					CDRom: &cnv.CDRomTarget{
						Bus: cnv.DiskBus(bus),
					},
				},
			}
		case QCOW2, RAW:
			disk = cnv.Disk{
				Name:      cnvVolumeName,
				BootOrder: bootOrder,
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

	// If bootOrder wasn't set by a bootable volume, set it to the image (if exists)
	if !bootOrderSet && imagePVC != nil {
		r.Log.Info("No bootable volume found, falling back to image", "image", imagePVC.Name)
		for i, disk := range kDisks {
			if disk.Name == fmt.Sprintf("vol-%s", imagePVC.Annotations[planbase.AnnDiskSource]) {
				kDisks[i].BootOrder = ptr.To[uint](1)
				r.Log.Info("Boot order set to 1 on", "disk", kDisks[i], "ann", imagePVC.Annotations[planbase.AnnDiskSource])
				break
			}
		}
	}

	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

func (r *Builder) mapNetworks(vm *model.Workload, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)
	numNetworks := 0
	for vmNetworkName, vmAddresses := range vm.Addresses {
		if nics, ok := vmAddresses.([]interface{}); ok {
			// Use only first NIC in order to avoid duplicates in case of multiple NICs per interface
			if len(nics) > 0 {
				nic := nics[0]
				// Look for the network map for the source network
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

				// Skip network mappings with destination type 'Ignored'
				if networkPair.Destination.Type == Ignored {
					continue
				}

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
						if !hasUDN || settings.Settings.UdnSupportsMac {
							kInterface.MacAddress = macAddress.(string)
						}
					}
					if ipType, ok := m["OS-EXT-IPS:type"]; ok {
						if ipType.(string) == "floating" {
							continue
						}
					}
				}

				switch networkPair.Destination.Type {
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
		err = liberr.Wrap(err, "vm", vmRef.String())
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

func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	return nil, nil
}

func (r *Builder) ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(vmRef ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	vm := &model.Workload{}
	if err = r.Source.Inventory.Find(vm, vmRef); err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	os, version, distro := r.getOs(vm)
	name = getPreferenceOs(os, version, distro)
	return
}

func (r *Builder) getOs(vm *model.Workload) (os, version, distro string) {
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
	return
}

func getPreferenceOs(os, version, distro string) string {
	if os != UnknownOS && version != "" {
		if distro == CentOS && len(version) >= 1 && (version[:1] == "8" || version[:1] == "9") {
			os = fmt.Sprintf("%s.stream%s", distro, version)
		} else if os == Windows {
			switch {
			case strings.Contains(version, "2k12") || strings.Contains(version, "2012"):
				os = fmt.Sprintf("%s.2k12.virtio", os)
			case strings.Contains(version, "2k16") || strings.Contains(version, "2016"):
				os = fmt.Sprintf("%s.2k16.virtio", os)
			case strings.Contains(version, "2k19") || strings.Contains(version, "2019"):
				os = fmt.Sprintf("%s.2k19.virtio", os)
			case strings.Contains(version, "2k22") || strings.Contains(version, "2022"):
				os = fmt.Sprintf("%s.2k22.virtio", os)
			case len(version) >= 2 && (version[:2] == "10" || version[:2] == "11"):
				os = fmt.Sprintf("%s.%s.virtio", os, version)
			default:
				os = "windows.10.virtio"
			}
		} else if distro == RHEL {
			os = fmt.Sprintf("%s.%s", os, version)
		}
	}
	return os
}

func getTemplateOs(os, version, distro string) string {
	if os != UnknownOS && version != "" {
		if distro == CentOS && len(version) >= 1 && (version[:1] == "8" || version[:1] == "9") {
			os = fmt.Sprintf("%s-stream%s", distro, version)
		} else if os == Windows {
			switch {
			case strings.Contains(version, "2k12") || strings.Contains(version, "2012"):
				os = fmt.Sprintf("%s2k12", os)
			case strings.Contains(version, "2k16") || strings.Contains(version, "2016"):
				os = fmt.Sprintf("%s2k16", os)
			case strings.Contains(version, "2k19") || strings.Contains(version, "2019"):
				os = fmt.Sprintf("%s2k19", os)
			case strings.Contains(version, "2k22") || strings.Contains(version, "2022"):
				os = fmt.Sprintf("%s2k22", os)
			case len(version) >= 2 && (version[:2] == "10" || version[:2] == "11"):
				os = fmt.Sprintf("%s%s", os, version)
			default:
				os = DefaultWindows
			}
		} else {
			os = fmt.Sprintf("%s%s", os, version)
		}
	}
	return os
}

// Build tasks.
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	vm := &model.Workload{}
	if err = r.Source.Inventory.Find(vm, vmRef); err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	tempOs, version, distro := r.getOs(vm)
	os := getTemplateOs(tempOs, version, distro)

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

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
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
		if imageID, ok := image.Properties[forkliftPropertyOriginalImageID]; ok && imageID == workload.ImageID {
			if image.DiskFormat != "raw" {
				r.Log.Info("this image will require conversion as it's not raw", "image", image.Name, "diskFormat", image.DiskFormat)
				annotations[planbase.AnnRequiresConversion] = "true"
				annotations[planbase.AnnSourceFormat] = image.DiskFormat
			}
		}
		if image.Status != string(ImageStatusActive) {
			r.Log.Info("the image is not ready yet", "image", image.Name, "status", image.Status)
			continue
		}
		if pvc, pvcErr := r.getCorrespondingPvc(image, workload, annotations, secretName); pvcErr == nil {
			pvcs = append(pvcs, pvc)
		} else {
			err = pvcErr
			return
		}
	}
	return
}

func (r *Builder) getCorrespondingPvc(image model.Image, workload *model.Workload, annotations map[string]string, secretName string) (pvc *core.PersistentVolumeClaim, err error) {
	populatorCR, err := r.ensureVolumePopulator(workload, &image, secretName)
	if err != nil {
		return
	}
	return r.ensureVolumePopulatorPVC(workload, &image, annotations, populatorCR.Name)
}

func (r *Builder) ensureVolumePopulator(workload *model.Workload, image *model.Image, secretName string) (populatorCR *api.OpenstackVolumePopulator, err error) {
	volumePopulatorCR, err := r.getVolumePopulatorCR(image.ID)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			err = liberr.Wrap(err)
			return
		}
		return r.createVolumePopulatorCR(*image, secretName, workload.ID)
	}
	populatorCR = &volumePopulatorCR
	return
}

func (r *Builder) ensureVolumePopulatorPVC(workload *model.Workload, image *model.Image, annotations map[string]string, populatorName string) (pvc *core.PersistentVolumeClaim, err error) {
	if pvc, err = r.getVolumePopulatorPVC(image.ID); err != nil {
		if !k8serr.IsNotFound(err) {
			err = liberr.Wrap(err)
			return
		}
		originalVolumeDiskId := image.Name
		if imageProperty, ok := image.Properties[forkliftPropertyOriginalVolumeID]; ok {
			originalVolumeDiskId = imageProperty.(string)
		}

		mapList := r.Context.Map.Storage.Spec.Map

		// Check if there's a storage map available
		if len(mapList) == 0 {
			err = liberr.New("no storage map found in the migration plan")
			return
		}

		var storageClassName string

		// VM is image based, look for a glance key in the mapping
		if workload.ImageID != "" {
			// At this point the StorageMap has been validated, and the VM has to be fully mapped
			for _, storageMap := range mapList {
				if storageMap.Source.Name == api.GlanceSource {
					storageClassName = storageMap.Destination.StorageClass
				}
			}
		} else {
			// VM has a volume, look for the volume type in the mapping
			if volumeType := r.getVolumeType(workload, originalVolumeDiskId); volumeType != "" {
				storageClassName, err = r.getStorageClassName(workload, volumeType)
				if err != nil {
					err = liberr.Wrap(err)
					return
				}
			}
		}

		if pvc, err = r.persistentVolumeClaimWithSourceRef(*image, storageClassName, populatorName, annotations, workload.ID); err != nil {
			err = liberr.Wrap(err)
			return
		}
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

func (r *Builder) createVolumePopulatorCR(image model.Image, secretName, vmId string) (populatorCR *api.OpenstackVolumePopulator, err error) {
	populatorCR = &api.OpenstackVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", image.Name),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels: r.Labeler.VMLabelsWithExtra(ref.Ref{ID: vmId}, map[string]string{
				"imageID": image.ID,
			}),
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
		err = liberr.Wrap(err)
		return
	}
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
		r.Log.Trace(err)
		return
	}
	for _, storageMap := range r.Context.Map.Storage.Spec.Map {
		if storageMap.Source.ID == volumeTypeID || storageMap.Source.Name == volumeTypeName {
			storageClassName = storageMap.Destination.StorageClass
		}
	}
	if storageClassName == "" {
		err = liberr.New("no storage class map found for volume type", "volumeTypeID", volumeTypeID)
		r.Log.Trace(err)
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
		return nil, nil, liberr.Wrap(err, "storageClassName", storageClassName)
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

// Get the OpenstackVolumePopulator CustomResource based on the image ID.
func (r *Builder) getVolumePopulatorCR(imageID string) (populatorCr api.OpenstackVolumePopulator, err error) {
	populatorCrList := &api.OpenstackVolumePopulatorList{}
	err = r.Destination.Client.List(context.TODO(), populatorCrList, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": getMigrationID(r.Context),
			"imageID":   imageID,
		}),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(populatorCrList.Items) == 0 {
		err = k8serr.NewNotFound(api.SchemeGroupVersion.WithResource("OpenstackVolumePopulator").GroupResource(), imageID)
		return
	}
	if len(populatorCrList.Items) > 1 {
		err = liberr.New("multiple OpenstackVolumePopulator CRs found for image", "imageID", imageID)
		return
	}

	populatorCr = populatorCrList.Items[0]

	return
}

func (r *Builder) getVolumePopulatorPVC(imageID string) (populatorPvc *core.PersistentVolumeClaim, err error) {
	populatorPvcList := &core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(context.TODO(), populatorPvcList, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": getMigrationID(r.Context),
			"imageID":   imageID,
		}),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(populatorPvcList.Items) == 0 {
		err = k8serr.NewNotFound(api.SchemeGroupVersion.WithResource("PersistentVolumeClaim").GroupResource(), imageID)
		return
	}
	if len(populatorPvcList.Items) > 1 {
		err = liberr.New("multiple PersistentVolumeClaims found for image", "imageID", imageID)
		return
	}

	populatorPvc = &populatorPvcList.Items[0]
	return
}

func (r *Builder) persistentVolumeClaimWithSourceRef(image model.Image,
	storageClassName string,
	populatorName string,
	annotations map[string]string,
	vmID string) (pvc *core.PersistentVolumeClaim, err error) {

	apiGroup := "forklift.konveyor.io"
	virtualSize := image.VirtualSize
	// virtual_size may not always be available
	if virtualSize == 0 {
		virtualSize = image.SizeBytes
	}

	var accessModes []core.PersistentVolumeAccessMode
	accessModes, volumeMode, err := r.getVolumeAndAccessMode(storageClassName)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	virtualSize = utils.CalculateSpaceWithOverhead(virtualSize, volumeMode)

	// The image might be a VM Snapshot Image and has no volume associated to it
	if originalVolumeDiskId, ok := image.Properties["forklift_original_volume_id"]; ok {
		annotations[planbase.AnnDiskSource] = originalVolumeDiskId.(string)
		r.Log.Info("the image comes from a volume", "volumeID", originalVolumeDiskId)
	} else if originalImageId, ok := image.Properties["forklift_original_image_id"]; ok {
		annotations[planbase.AnnDiskSource] = originalImageId.(string)
		r.Log.Info("the image comes from a vm snapshot", "imageID", originalImageId)
	} else {
		r.Log.Error(nil, "the image has no volume or vm snapshot associated to it", "image", image.Name)
	}

	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", image.ID),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Annotations:  annotations,
			Labels: r.Labeler.VMLabelsWithExtra(ref.Ref{ID: vmID}, map[string]string{
				"imageID": image.ID,
			}),
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: core.VolumeResourceRequirements{
				Requests: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: *resource.NewQuantity(virtualSize, resource.BinarySI)},
			},
			StorageClassName: &storageClassName,
			VolumeMode:       volumeMode,
			DataSourceRef: &core.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     api.OpenstackVolumePopulatorKind,
				Name:     populatorName,
			},
		},
	}

	err = r.Client.Create(context.TODO(), pvc, &client.CreateOptions{})
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (r *Builder) PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	image, err := r.getImageFromPVC(persistentVolumeClaim)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	populatorCr, err := r.getVolumePopulatorCR(image.ID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	progressPercentage, err := strconv.ParseInt(populatorCr.Status.Progress, 10, 64)
	if err != nil {
		transferredBytes = 0
		err = nil
		//nolint:nilerr
		return
	}

	pvcSize := persistentVolumeClaim.Spec.Resources.Requests["storage"]
	transferredBytes = (progressPercentage * pvcSize.Value()) / 100
	return
}

// Get the Openstack image from the inventory based on the PVC.
func (r *Builder) getImageFromPVC(pvc *core.PersistentVolumeClaim) (image *model.Image, err error) {
	image = &model.Image{}
	err = r.Source.Inventory.Find(image, ref.Ref{ID: pvc.Labels["imageID"]})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	workload := &model.Workload{}
	err = r.Source.Inventory.Find(workload, vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var images []*model.Image
	for _, volume := range workload.Volumes {
		lookupName := getImageFromVolumeName(r.Context, vmRef.ID, volume.ID)
		image, err := r.getImageByName(lookupName)
		if err != nil {
			r.Log.Error(err, "Couldn't find the image from the volume.", "volume", volume.ID, "vmRef", vmRef)
			continue
		}
		images = append(images, image)
	}
	if len(images) != len(pvcs) {
		// To be sure we have every disk based on what already migrated and what's not.
		// e.g when initializing the plan and the PVC has not been created yet (but the populator CR is) or when the disks that are attached to the source VM change.
		for _, pvc := range pvcs {
			image, err := r.getImageFromPVC(pvc)
			if err != nil {
				continue
			}
			images = append(images, image)
		}
	}
	migrationID := r.ActiveMigrationUID()
	for _, image := range images {
		populatorCr, err := r.getVolumePopulatorCR(image.ID)
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

// ConversionPodConfig returns provider-specific configuration for the virt-v2v conversion pod.
// OpenStack provider does not require any special configuration.
func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}
