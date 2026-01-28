package builder

import (
	"fmt"
	"path"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
)

// Network types
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// Input types
const (
	Tablet = "tablet"
)

// Bus types for compatibility mode
const (
	Virtio = "virtio"
	E1000e = "e1000e"
)

// Template labels
const (
	TemplateOSLabel       = "os.template.kubevirt.io/%s"
	TemplateWorkloadLabel = "workload.template.kubevirt.io/server"
	TemplateFlavorLabel   = "flavor.template.kubevirt.io/medium"
)

// Operating Systems for template labels
const (
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	Unknown        = "unknown"
)

// VirtualMachine builds a KubeVirt VirtualMachine spec from an EC2 instance.
// Maps EC2 instance type to CPU/memory, attaches PVCs as disks, configures networking,
// and sets firmware/boot options based on EC2 instance properties.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	awsInstance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		return err
	}

	name := inventory.GetInstanceName(awsInstance)

	instanceType := string(awsInstance.InstanceType)
	if instanceType == "" {
		instanceType = "m5.large"
		r.log.Info("InstanceType not found, using default", "vm", name, "default", instanceType)
	}

	vcpus, memoryMiB := r.mapInstanceType(instanceType)

	runStrategy := cnv.RunStrategyHalted
	object.RunStrategy = &runStrategy
	object.Template = &cnv.VirtualMachineInstanceTemplateSpec{
		Spec: cnv.VirtualMachineInstanceSpec{
			Domain: cnv.DomainSpec{
				Resources: cnv.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memoryMiB)),
					},
				},
				Devices: cnv.Devices{
					Disks:      []cnv.Disk{},
					Interfaces: []cnv.Interface{},
					Inputs:     []cnv.Input{},
				},
			},
			Networks: []cnv.Network{},
			Volumes:  []cnv.Volume{},
		},
	}

	// Map CPU with topology (sockets/cores) and nested virt for metal instances
	r.mapCPU(vcpus, instanceType, object)

	// Map firmware (BIOS/EFI) and serial number
	r.mapFirmware(awsInstance, object)

	// Map input devices (tablet for better VNC experience)
	r.mapInput(object)

	// Map disks with boot order
	r.mapDisks(awsInstance, persistentVolumeClaims, object)

	// Map networks with Multus support and MAC preservation
	err = r.mapNetworks(awsInstance, object)
	if err != nil {
		return err
	}

	// Map node selector for AZ-based scheduling if enabled
	r.mapNodeSelector(awsInstance, object)

	return nil
}

// mapCPU configures the VM's CPU with proper topology.
// Uses sockets=1 with all cores, which is a common layout for cloud instances.
// For .metal instances, enables nested virtualization (vmx/svm features).
func (r *Builder) mapCPU(vcpus int32, instanceType string, object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: 1,
		Cores:   uint32(vcpus),
	}

	// Enable nested virtualization for bare metal instances
	// EC2 .metal instances support nested virtualization
	if r.isMetalInstance(instanceType) {
		var features []cnv.CPUFeature
		features = append(features, cnv.CPUFeature{
			Name:   "vmx",
			Policy: "optional",
		})
		features = append(features, cnv.CPUFeature{
			Name:   "svm",
			Policy: "optional",
		})
		object.Template.Spec.Domain.CPU.Features = features
		r.log.Info("Enabled nested virtualization for metal instance", "instanceType", instanceType)
	}
}

// isMetalInstance checks if the instance type is a bare metal instance.
// EC2 bare metal instances have ".metal" suffix (e.g., m5.metal, c5.metal).
func (r *Builder) isMetalInstance(instanceType string) bool {
	return strings.HasSuffix(instanceType, ".metal")
}

// mapFirmware configures BIOS/EFI firmware based on EC2 BootMode and sets system serial.
// EC2 BootMode values: "legacy-bios", "uefi", "uefi-preferred"
func (r *Builder) mapFirmware(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) {
	// Use instance ID as system serial number for identification
	serial := ""
	if awsInstance.InstanceId != nil {
		serial = *awsInstance.InstanceId
	}

	firmware := &cnv.Firmware{
		Serial: serial,
	}

	// Determine firmware type from EC2 BootMode
	bootMode := awsInstance.BootMode
	if bootMode == ec2types.BootModeValuesUefi || bootMode == ec2types.BootModeValuesUefiPreferred {
		// UEFI firmware
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: ptr.To(false), // EC2 doesn't expose secure boot status directly
			},
		}
		// SMM is required for UEFI
		object.Template.Spec.Domain.Features = &cnv.Features{
			ACPI: cnv.FeatureState{},
			SMM:  &cnv.FeatureState{Enabled: ptr.To(true)},
		}
	} else {
		// Default to BIOS (legacy-bios or not specified)
		firmware.Bootloader = &cnv.Bootloader{
			BIOS: &cnv.BIOS{},
		}
		// ACPI is needed for proper shutdown
		object.Template.Spec.Domain.Features = &cnv.Features{
			ACPI: cnv.FeatureState{},
		}
	}

	object.Template.Spec.Domain.Firmware = firmware
}

// mapInput adds input devices for better user experience.
// Tablet input is essential for proper mouse cursor tracking in VNC/console.
// In compatibility mode, uses USB bus instead of Virtio.
func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	bus := cnv.InputBusVirtio
	if r.useCompatibilityMode() {
		bus = cnv.InputBusUSB
	}
	tablet := cnv.Input{
		Type: Tablet,
		Name: Tablet,
		Bus:  bus,
	}
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{tablet}
}

// mapDisks attaches PVCs as disks and sets boot order.
// The root device (first EBS volume) is set as the primary boot device.
// In compatibility mode, uses SATA bus instead of Virtio.
func (r *Builder) mapDisks(awsInstance *model.InstanceDetails, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	pvcByVolumeID := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range persistentVolumeClaims {
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			pvcByVolumeID[volumeID] = pvc
		}
	}

	// Select disk bus based on compatibility mode
	bus := cnv.DiskBusVirtio
	if r.useCompatibilityMode() {
		bus = cnv.DiskBusSATA
	}

	diskIndex := 0
	for _, dev := range awsInstance.BlockDeviceMappings {
		if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
			continue
		}

		volumeID := *dev.Ebs.VolumeId

		pvc, pvcFound := pvcByVolumeID[volumeID]
		if !pvcFound {
			r.log.Info("No PVC found for volume, skipping", "vm", awsInstance.Name, "volumeID", volumeID)
			continue
		}

		diskName := fmt.Sprintf("disk-%d", diskIndex)

		disk := cnv.Disk{
			Name: diskName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: bus,
				},
			},
		}

		// Set boot order - first disk is the boot device
		if diskIndex == 0 {
			disk.BootOrder = ptr.To(uint(1))
		}

		object.Template.Spec.Domain.Devices.Disks = append(
			object.Template.Spec.Domain.Devices.Disks,
			disk,
		)

		object.Template.Spec.Volumes = append(
			object.Template.Spec.Volumes,
			cnv.Volume{
				Name: diskName,
				VolumeSource: cnv.VolumeSource{
					PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
		)

		diskIndex++
	}
}

// mapNetworks configures VM networks based on EC2 network interfaces and network mappings.
// Supports pod networking, Multus, and UDN (User Defined Networks).
// Preserves MAC addresses from source (when UDN supports it or when not using UDN).
// In compatibility mode, uses E1000e NIC model instead of VirtIO.
func (r *Builder) mapNetworks(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) error {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	// Check if destination cluster has UDN
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)

	// Select NIC model based on compatibility mode
	interfaceModel := Virtio
	if r.useCompatibilityMode() {
		interfaceModel = E1000e
	}

	// Get network interfaces from the EC2 instance
	networkInterfaces := awsInstance.NetworkInterfaces
	if len(networkInterfaces) == 0 {
		// No network interfaces - add default pod network
		r.log.Info("No network interfaces found, using default pod network", "vm", awsInstance.Name)
		kNetwork := cnv.Network{
			Name: "default",
			NetworkSource: cnv.NetworkSource{
				Pod: &cnv.PodNetwork{},
			},
		}
		kInterface := cnv.Interface{
			Name:  "default",
			Model: interfaceModel,
		}

		if hasUDN {
			kInterface.Binding = &cnv.PluginBinding{
				Name: planbase.UdnL2bridge,
			}
		} else {
			kInterface.InterfaceBindingMethod = cnv.InterfaceBindingMethod{
				Masquerade: &cnv.InterfaceMasquerade{},
			}
		}

		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	} else {
		// Map each network interface using the network mapping
		netMapIn := r.Context.Map.Network.Spec.Map
		networkIndex := 0

		for _, eni := range networkInterfaces {
			// Find mapping for this subnet
			var mapped *api.NetworkPair
			if eni.SubnetId != nil {
				mapped = r.findNetworkMapping(*eni.SubnetId, netMapIn)
			}

			// Skip if destination type is Ignored
			if mapped != nil && mapped.Destination.Type == Ignored {
				continue
			}

			networkName := fmt.Sprintf("net-%d", networkIndex)
			kNetwork := cnv.Network{Name: networkName}
			kInterface := cnv.Interface{
				Name:  networkName,
				Model: interfaceModel,
			}

			// Preserve MAC address from source ENI
			// When UDN is enabled, only preserve MAC if UDN supports it
			if eni.MacAddress != nil && *eni.MacAddress != "" {
				if !hasUDN || settings.Settings.UdnSupportsMac {
					kInterface.MacAddress = *eni.MacAddress
				}
			}

			if mapped == nil || mapped.Destination.Type == Pod {
				// Default to pod networking
				kNetwork.Pod = &cnv.PodNetwork{}
				if hasUDN {
					kInterface.Binding = &cnv.PluginBinding{
						Name: planbase.UdnL2bridge,
					}
				} else {
					kInterface.InterfaceBindingMethod = cnv.InterfaceBindingMethod{
						Masquerade: &cnv.InterfaceMasquerade{},
					}
				}
			} else if mapped.Destination.Type == Multus {
				// Multus network
				kNetwork.Multus = &cnv.MultusNetwork{
					NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
				}
				kInterface.InterfaceBindingMethod = cnv.InterfaceBindingMethod{
					Bridge: &cnv.InterfaceBridge{},
				}
			}

			kNetworks = append(kNetworks, kNetwork)
			kInterfaces = append(kInterfaces, kInterface)
			networkIndex++
		}
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces

	return nil
}

// TopologyZoneLabel is the standard Kubernetes topology label for availability zones.
// AWS EKS and OpenShift on AWS automatically label nodes with this key.
const TopologyZoneLabel = "topology.kubernetes.io/zone"

// mapNodeSelector sets node selector for AZ-based scheduling (enabled by default).
// Uses the standard Kubernetes topology label (topology.kubernetes.io/zone) to schedule VMs
// on nodes in the provider's configured target-az (where the migrated EBS volumes are created).
func (r *Builder) mapNodeSelector(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) {
	// Skip if SkipZoneNodeSelector is explicitly set to true in the Plan
	if r.Plan.Spec.SkipZoneNodeSelector {
		return
	}

	// Get the target-az from provider settings (where volumes are created)
	az, err := r.getTargetAZ()
	if err != nil {
		r.log.Info("Could not get target AZ from provider settings, skipping zone node selector",
			"vm", awsInstance.Name,
			"error", err.Error())
		return
	}

	// Initialize node selector if not set
	if object.Template.Spec.NodeSelector == nil {
		object.Template.Spec.NodeSelector = make(map[string]string)
	}

	// Set the topology zone label to match the provider's target AZ
	object.Template.Spec.NodeSelector[TopologyZoneLabel] = az
	r.log.Info("Set AZ-based node selector for VM",
		"vm", awsInstance.Name,
		"targetAZ", az,
		"label", TopologyZoneLabel)
}

// getTargetAZ retrieves the target availability zone from provider settings.
// This is the AZ where EBS volumes are created and where VMs should be scheduled.
func (r *Builder) getTargetAZ() (string, error) {
	if r.Source.Provider == nil {
		return "", fmt.Errorf("source provider is nil")
	}

	if r.Source.Provider.Spec.Settings == nil {
		return "", fmt.Errorf("provider spec.settings is not configured")
	}

	targetAZ, ok := r.Source.Provider.Spec.Settings["target-az"]
	if !ok || targetAZ == "" {
		return "", fmt.Errorf("provider spec.settings.target-az is not configured")
	}

	return targetAZ, nil
}

// mapInstanceType extracts the size suffix from an EC2 instance type and returns corresponding resources.
// Parses "family.size" format (e.g., "m5.large") and looks up vCPU/memory from instanceSizeSpecs.
// Defaults to 2 vCPUs and 4096 MiB if parsing fails.
func (r *Builder) mapInstanceType(instanceType string) (vcpus int32, memoryMiB int64) {
	vcpus = 2
	memoryMiB = 4096

	if len(instanceType) > 0 {
		parts := strings.Split(instanceType, ".")
		if len(parts) > 1 {
			size := parts[len(parts)-1]
			vcpus, memoryMiB = r.mapInstanceSize(size)
		}
	}

	r.log.V(1).Info("Mapped instance type", "type", instanceType, "vcpus", vcpus, "memoryMiB", memoryMiB)
	return
}

// instanceSizeSpec defines the resource allocation (CPU and memory) for an EC2 instance size.
//
// EC2 instance types follow a naming pattern: family.size (e.g., m5.large, t3.xlarge).
// This struct stores the vCPU count and memory allocation for each size suffix.
type instanceSizeSpec struct {
	// vcpus is the number of virtual CPUs allocated to this instance size.
	// Maps to the KubeVirt VirtualMachine's CPU cores specification.
	vcpus int32

	// memoryMiB is the amount of memory in mebibytes (MiB) allocated to this instance size.
	// Maps to the KubeVirt VirtualMachine's memory request specification.
	memoryMiB int64
}

// instanceSizeSpecs maps EC2 instance size suffixes to CPU/memory for KubeVirt resource requests.
// Instance types: <family>.<size> (e.g., t3.medium, m5.xlarge). Memory in MiB (1024-based).
var instanceSizeSpecs = map[string]instanceSizeSpec{
	"nano":     {1, 512},      // 1 vCPU, 512 MiB (0.5 GiB) - minimal instances
	"micro":    {1, 1024},     // 1 vCPU, 1 GiB - t2.micro, t3.micro
	"small":    {1, 2048},     // 1 vCPU, 2 GiB - t2.small, t3.small
	"medium":   {2, 4096},     // 2 vCPU, 4 GiB - t3.medium, m5.medium
	"large":    {2, 8192},     // 2 vCPU, 8 GiB - t3.large, m5.large
	"xlarge":   {4, 16384},    // 4 vCPU, 16 GiB - m5.xlarge, c5.xlarge
	"2xlarge":  {8, 32768},    // 8 vCPU, 32 GiB - m5.2xlarge
	"4xlarge":  {16, 65536},   // 16 vCPU, 64 GiB - m5.4xlarge
	"8xlarge":  {32, 131072},  // 32 vCPU, 128 GiB - m5.8xlarge
	"12xlarge": {48, 196608},  // 48 vCPU, 192 GiB - m5.12xlarge
	"16xlarge": {64, 262144},  // 64 vCPU, 256 GiB - m5.16xlarge
	"24xlarge": {96, 393216},  // 96 vCPU, 384 GiB - m5.24xlarge
	"32xlarge": {128, 524288}, // 128 vCPU, 512 GiB - largest instances like m5.32xlarge
}

// mapInstanceSize looks up CPU and memory allocation for an EC2 instance size suffix.
// Returns values from instanceSizeSpecs map or defaults to 2 vCPUs/4096 MiB for unknown sizes.
func (r *Builder) mapInstanceSize(size string) (vcpus int32, memoryMiB int64) {
	if spec, ok := instanceSizeSpecs[size]; ok {
		return spec.vcpus, spec.memoryMiB
	}

	r.log.Info("Unknown instance size, using default", "size", size)
	return 2, 4096
}

// useCompatibilityMode checks if compatibility mode should be used.
// Compatibility mode uses SATA disks, E1000e NICs, and USB input instead of Virtio.
// This is useful for guest OSes that don't have Virtio drivers.
func (r *Builder) useCompatibilityMode() bool {
	return r.Plan.Spec.SkipGuestConversion && r.Plan.Spec.UseCompatibilityMode
}

// TemplateLabels returns OS-specific template labels for the VM.
// Detects Windows vs Linux from EC2 platform details.
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	awsInstance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	// Detect OS from EC2 platform details
	os := r.detectOS(awsInstance)

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return labels, nil
}

// detectOS determines the operating system from EC2 instance metadata.
// Uses Platform (windows) and PlatformDetails for more specific detection.
func (r *Builder) detectOS(awsInstance *model.InstanceDetails) string {
	// Check Platform field first (only set for Windows)
	if awsInstance.Platform == ec2types.PlatformValuesWindows {
		return DefaultWindows
	}

	// Check PlatformDetails for more specific OS info
	if awsInstance.PlatformDetails != nil {
		details := strings.ToLower(*awsInstance.PlatformDetails)

		// Windows detection
		if strings.Contains(details, "windows") {
			if strings.Contains(details, "2022") {
				return "win2k22"
			}
			if strings.Contains(details, "2019") {
				return "win2k19"
			}
			if strings.Contains(details, "2016") {
				return "win2k16"
			}
			if strings.Contains(details, "2012") {
				return "win2k12r2"
			}
			return DefaultWindows
		}

		// Linux detection
		if strings.Contains(details, "red hat") || strings.Contains(details, "rhel") {
			if strings.Contains(details, "9") {
				return "rhel9.0"
			}
			if strings.Contains(details, "8") {
				return "rhel8.1"
			}
			return "rhel8.1"
		}

		if strings.Contains(details, "ubuntu") {
			return "ubuntu20.04"
		}

		if strings.Contains(details, "centos") {
			return "centos8"
		}

		if strings.Contains(details, "debian") {
			return "debian10"
		}

		if strings.Contains(details, "fedora") {
			return "fedora31"
		}

		if strings.Contains(details, "amazon linux") || strings.Contains(details, "al2") {
			return "rhel8.1" // Amazon Linux is RHEL-based
		}

		if strings.Contains(details, "suse") || strings.Contains(details, "sles") {
			return "opensuse15.0"
		}

		// Generic Linux
		if strings.Contains(details, "linux") {
			return DefaultLinux
		}
	}

	// Default to Linux
	return DefaultLinux
}

// ConversionPodConfig returns zone-based configuration for the virt-v2v conversion pod.
// This ensures the conversion pod runs on a node in the same AZ as the EBS volumes,
// which is required for volume attachment by the EBS CSI driver.
func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	if r.Plan.Spec.SkipZoneNodeSelector {
		r.log.V(1).Info("Skipping zone node selector for conversion pod (SkipZoneNodeSelector=true)")
		return &planbase.ConversionPodConfigResult{}, nil
	}

	az, err := r.getTargetAZ()
	if err != nil {
		r.log.Info("Could not get target AZ, skipping conversion pod zone selector", "error", err.Error())
		return &planbase.ConversionPodConfigResult{}, nil
	}

	r.log.Info("Setting zone-based node selector for conversion pod", "targetAZ", az)
	return &planbase.ConversionPodConfigResult{
		NodeSelector: map[string]string{
			TopologyZoneLabel: az,
		},
	}, nil
}
