package base

import (
	"fmt"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Network types.
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// Input types.
const (
	Tablet = "tablet"
)

// Bus types.
const (
	Virtio = "virtio"
	E1000e = "e1000e"
)

// Template labels.
const (
	TemplateOSLabel       = "os.template.kubevirt.io/%s"
	TemplateWorkloadLabel = "workload.template.kubevirt.io/server"
	TemplateFlavorLabel   = "flavor.template.kubevirt.io/medium"
)

// Operating system identifiers for template labels.
const (
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	Unknown        = "unknown"
)

// TopologyZoneLabel is the standard Kubernetes topology label for availability zones.
const TopologyZoneLabel = "topology.kubernetes.io/zone"

// Base holds the shared state and helpers used by both the imperative and
// template VM builders. It embeds plancontext.Context and satisfies every
// method of the base.Builder interface except VirtualMachine.
type Base struct {
	*plancontext.Context
	Log logging.LevelLogger
}

// New creates a Base with plan context and a default logger.
func New(ctx *plancontext.Context) *Base {
	return &Base{
		Context: ctx,
		Log:     logging.WithName("builder|ec2"),
	}
}

// UseCompatibilityMode returns true when SATA/E1000e/USB should replace Virtio.
func (r *Base) UseCompatibilityMode() bool {
	return r.Plan.Spec.SkipGuestConversion && r.Plan.Spec.UseCompatibilityMode
}

// IsMetalInstance returns true for bare metal instance types (e.g. m5.metal).
func (r *Base) IsMetalInstance(instanceType string) bool {
	return strings.HasSuffix(instanceType, ".metal")
}

// ResolveInstanceType returns the instance type string, defaulting to "m5.large".
func (r *Base) ResolveInstanceType(awsInstance *model.InstanceDetails) string {
	instanceType := string(awsInstance.InstanceType)
	if instanceType == "" {
		instanceType = "m5.large"
		r.Log.Info("InstanceType not found, using default", "default", instanceType)
	}
	return instanceType
}

// GetTargetAZ retrieves the target availability zone from provider settings.
func (r *Base) GetTargetAZ() (string, error) {
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

// InstanceSizeSpec defines CPU/memory for an EC2 instance size suffix.
type InstanceSizeSpec struct {
	Vcpus     int32
	MemoryMiB int64
}

// InstanceSizeSpecs maps EC2 size suffixes to CPU/memory.
var InstanceSizeSpecs = map[string]InstanceSizeSpec{
	"nano":     {1, 512},
	"micro":    {1, 1024},
	"small":    {1, 2048},
	"medium":   {2, 4096},
	"large":    {2, 8192},
	"xlarge":   {4, 16384},
	"2xlarge":  {8, 32768},
	"4xlarge":  {16, 65536},
	"8xlarge":  {32, 131072},
	"12xlarge": {48, 196608},
	"16xlarge": {64, 262144},
	"24xlarge": {96, 393216},
	"32xlarge": {128, 524288},
}

// MapInstanceType parses "family.size" and returns vCPUs/memory.
// Defaults to 2 vCPUs and 4096 MiB.
func (r *Base) MapInstanceType(instanceType string) (vcpus int32, memoryMiB int64) {
	vcpus, memoryMiB = 2, 4096
	if len(instanceType) > 0 {
		parts := strings.Split(instanceType, ".")
		if len(parts) > 1 {
			vcpus, memoryMiB = r.MapInstanceSize(parts[len(parts)-1])
		}
	}
	r.Log.V(1).Info("Mapped instance type", "type", instanceType, "vcpus", vcpus, "memoryMiB", memoryMiB)
	return
}

// MapInstanceSize looks up CPU/memory for a size suffix, defaulting to 2/4096.
func (r *Base) MapInstanceSize(size string) (vcpus int32, memoryMiB int64) {
	if spec, ok := InstanceSizeSpecs[size]; ok {
		return spec.Vcpus, spec.MemoryMiB
	}
	r.Log.Info("Unknown instance size, using default", "size", size)
	return 2, 4096
}

// DetectOS determines the operating system from EC2 instance metadata.
func (r *Base) DetectOS(awsInstance *model.InstanceDetails) string {
	if awsInstance.Platform == ec2types.PlatformValuesWindows {
		return DefaultWindows
	}
	if awsInstance.PlatformDetails != nil {
		details := strings.ToLower(*awsInstance.PlatformDetails)

		if strings.Contains(details, "windows") {
			switch {
			case strings.Contains(details, "2022"):
				return "win2k22"
			case strings.Contains(details, "2019"):
				return "win2k19"
			case strings.Contains(details, "2016"):
				return "win2k16"
			case strings.Contains(details, "2012"):
				return "win2k12r2"
			default:
				return DefaultWindows
			}
		}

		switch {
		case strings.Contains(details, "red hat") || strings.Contains(details, "rhel"):
			if strings.Contains(details, "9") {
				return "rhel9.0"
			}
			return "rhel8.1"
		case strings.Contains(details, "ubuntu"):
			return "ubuntu20.04"
		case strings.Contains(details, "centos"):
			return "centos8"
		case strings.Contains(details, "debian"):
			return "debian10"
		case strings.Contains(details, "fedora"):
			return "fedora31"
		case strings.Contains(details, "amazon linux") || strings.Contains(details, "al2"):
			return "rhel8.1"
		case strings.Contains(details, "suse") || strings.Contains(details, "sles"):
			return "opensuse15.0"
		case strings.Contains(details, "linux"):
			return DefaultLinux
		}
	}
	return DefaultLinux
}

// TemplateLabels returns OS-specific template labels for the VM.
func (r *Base) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	awsInstance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}
	os := r.DetectOS(awsInstance)
	return r.BuildTemplateLabels(os), nil
}

// BuildTemplateLabels returns OpenShift template matching labels.
func (r *Base) BuildTemplateLabels(osType string) map[string]string {
	os := osType
	if os == "" {
		os = DefaultLinux
	}
	return map[string]string{
		fmt.Sprintf(TemplateOSLabel, os): "true",
		TemplateWorkloadLabel:            "true",
		TemplateFlavorLabel:              "true",
	}
}

// ConversionPodConfig returns zone-based configuration for the conversion pod.
func (r *Base) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	if r.Plan.Spec.SkipZoneNodeSelector {
		r.Log.V(1).Info("Skipping zone node selector for conversion pod (SkipZoneNodeSelector=true)")
		return &planbase.ConversionPodConfigResult{}, nil
	}
	az, err := r.GetTargetAZ()
	if err != nil {
		r.Log.Info("Could not get target AZ, skipping conversion pod zone selector", "error", err.Error())
		return &planbase.ConversionPodConfigResult{}, nil
	}
	r.Log.Info("Setting zone-based node selector for conversion pod", "targetAZ", az)
	return &planbase.ConversionPodConfigResult{
		NodeSelector: map[string]string{
			TopologyZoneLabel: az,
		},
	}, nil
}

// IsUEFI returns true when the EC2 instance uses UEFI firmware.
func (r *Base) IsUEFI(awsInstance *model.InstanceDetails) bool {
	return awsInstance.BootMode == ec2types.BootModeValuesUefi ||
		awsInstance.BootMode == ec2types.BootModeValuesUefiPreferred
}
