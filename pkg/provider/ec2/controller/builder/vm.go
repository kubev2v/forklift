package builder

import (
	"fmt"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	builder "github.com/kubev2v/forklift/pkg/provider/builder"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	core "k8s.io/api/core/v1"
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
// Uses a three-phase pipeline:
//  1. Extract: reads the EC2 instance and resolves PVCs/network mappings into VMBuildValues
//  2. Render: executes a Go text/template (default or custom from ConfigMap) with the values
//  3. Unmarshal: parses the rendered YAML into a cnv.VirtualMachineSpec
//
// Custom templates can be specified via PlanSpec.VMTemplate (a ConfigMap reference).
// The default template reproduces the exact output of the previous map*-based builder.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	// Phase 1: Extract values from source EC2 instance
	values, err := r.extractValues(vmRef, persistentVolumeClaims)
	if err != nil {
		return err
	}

	// Phase 2: Load template (custom from ConfigMap or built-in default)
	tmpl, err := r.loadVMTemplate()
	if err != nil {
		return err
	}

	// Phase 3: Render template with values → VirtualMachineSpec
	spec, err := builder.RenderTemplate(tmpl, values)
	if err != nil {
		return err
	}

	// Apply rendered spec to the output object
	*object = *spec
	return nil
}

// loadVMTemplate returns the Go text/template string to use for VM rendering.
// Currently customization is not supported.
// The built-in DefaultVMTemplate is returned.
func (r *Builder) loadVMTemplate() (string, error) {
	return DefaultVMTemplate, nil
}

// isMetalInstance checks if the instance type is a bare metal instance.
// EC2 bare metal instances have ".metal" suffix (e.g., m5.metal, c5.metal).
func (r *Builder) isMetalInstance(instanceType string) bool {
	return strings.HasSuffix(instanceType, ".metal")
}

// TopologyZoneLabel is the standard Kubernetes topology label for availability zones.
// AWS EKS and OpenShift on AWS automatically label nodes with this key.
const TopologyZoneLabel = "topology.kubernetes.io/zone"

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
