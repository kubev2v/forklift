package plan

import (
	"fmt"
	"path"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Plan hook.
type HookRef struct {
	// Pipeline step.
	Step string `json:"step"`
	// Hook reference.
	Hook core.ObjectReference `json:"hook" ref:"Hook"`
}

// TargetPowerState defines the desired power state of the target VM after migration
type TargetPowerState string

const (
	// Target power state constants
	TargetPowerStateOn   TargetPowerState = "on"
	TargetPowerStateOff  TargetPowerState = "off"
	TargetPowerStateAuto TargetPowerState = "auto"
)

func (r *HookRef) String() string {
	return fmt.Sprintf(
		"%s @%s",
		path.Join(r.Hook.Namespace, r.Hook.Name),
		r.Step)
}

// A VM listed on the plan.
type VM struct {
	ref.Ref `json:",inline"`
	// Enable hooks.
	Hooks []HookRef `json:"hooks,omitempty"`
	// Disk decryption LUKS keys
	// +optional
	LUKS core.ObjectReference `json:"luks" ref:"Secret"`
	// Attempt passphrase-less unlocking for all devices with Clevis, over the network.
	// Conversion pod running on target cluster will attempt to connect to a TANG server, make sure TANG
	// server is available on target network.
	// https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/configuring-automated-unlocking-of-encrypted-volumes-using-policy-based-decryption_security-hardening
	// If both nbdeClevis and LUKS are configured, nbdeClevis takes precedence.
	// +optional
	NbdeClevis bool `json:"nbdeClevis,omitempty"`
	// Choose the primary disk the VM boots from
	// +optional
	RootDisk string `json:"rootDisk,omitempty"`
	// Selected InstanceType that will override the VM properties.
	// +optional
	InstanceType string `json:"instanceType,omitempty"`
	// PVCNameTemplate is a template for generating PVC names for VM disks.
	// Generated names must be valid DNS-1123 labels (lowercase alphanumerics, '-' allowed, max 63 chars).
	// It follows Go template syntax and has access to provider-specific variables.
	//
	// Common variables (all providers):
	//   - .VmName: name of the VM in the source cluster (original source name)
	//   - .TargetVmName: final VM name in the target cluster (may equal .VmName if no rename/normalization)
	//   - .PlanName: name of the migration plan
	//   - .DiskIndex: initial volume index of the disk
	//
	// VMware (vSphere) specific variables:
	//   - .WinDriveLetter: Windows drive letter (lowercase, if applicable, e.g. "c", requires guest agent)
	//   - .RootDiskIndex: index of the root disk
	//   - .Shared: true if the volume is shared by multiple VMs, false otherwise
	//   - .FileName: name of the file in the source provider (filename includes the .vmdk suffix)
	//
	// OpenShift specific variables:
	//   - .SourcePVCName: name of the PVC in the source cluster
	//   - .SourcePVCNamespace: namespace of the PVC in the source cluster
	//
	// Note:
	//   This template overrides the plan level template.
	// Examples:
	//   "{{.TargetVmName}}-disk-{{.DiskIndex}}"
	//   "{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data{{end}}-{{.DiskIndex}}" (VMware)
	//   "{{.TargetVmName}}-{{.SourcePVCName}}" (OpenShift)
	// See:
	// 	 https://github.com/kubev2v/forklift/tree/main/pkg/templateutil for template functions.
	// +optional
	PVCNameTemplate string `json:"pvcNameTemplate,omitempty"`
	// VolumeNameTemplate is a template for generating volume interface names in the target virtual machine.
	// It follows Go template syntax and has access to the following variables:
	//   - .PVCName: name of the PVC mounted to the VM using this volume
	//   - .VolumeIndex: sequential index of the volume interface (0-based)
	//
	// Provider support:
	//   - VMware (vSphere): Supported. Default naming is "vol-{index}".
	//   - OpenShift: Not supported. Volume names are preserved from the source VM.
	//
	// Note:
	//   - This template will override at the plan level template
	//   - If not specified on VM level and on Plan level, default naming conventions will be used
	// Examples:
	//   "disk-{{.VolumeIndex}}"
	//   "pvc-{{.PVCName}}"
	// +optional
	VolumeNameTemplate string `json:"volumeNameTemplate,omitempty"`
	// NetworkNameTemplate is a template for generating network interface names in the target virtual machine.
	// It follows Go template syntax and has access to the following variables:
	//   - .NetworkName: If target network is multus, name of the Multus network attachment definition, empty otherwise.
	//   - .NetworkNamespace: If target network is multus, namespace where the network attachment definition is located.
	//   - .NetworkType: type of the network ("Multus" or "Pod")
	//   - .NetworkIndex: sequential index of the network interface (0-based)
	// The template can be used to customize network interface names based on target network configuration.
	//
	// Provider support:
	//   - VMware (vSphere): Supported. Network interface names can be customized.
	//   - OpenShift: Not supported. Network interface names are preserved from the source VM.
	//
	// Note:
	//   - This template will override at the plan level template
	//   - If not specified on VM level and on Plan level, default naming conventions will be used
	// Examples:
	//   "net-{{.NetworkIndex}}"
	//   "{{if eq .NetworkType "Pod"}}pod{{else}}multus-{{.NetworkIndex}}{{end}}"
	// +optional
	NetworkNameTemplate string `json:"networkNameTemplate,omitempty"`
	// TargetName specifies a custom name for the VM in the target cluster.
	// If not provided, the original VM name will be used and automatically adjusted to meet k8s DNS1123 requirements.
	// If provided, this exact name will be used instead. The migration will fail if the name is not unique or already in use.
	// +optional
	TargetName string `json:"targetName,omitempty"`
	// TargetPowerState specifies the desired power state of the target VM after migration.
	// - "on": Target VM will be powered on after migration
	// - "off": Target VM will be powered off after migration
	// - "auto" or nil (default): Target VM will match the source VM's power state
	// +optional
	// +kubebuilder:validation:Enum=on;off;auto
	TargetPowerState TargetPowerState `json:"targetPowerState,omitempty"`
	// DeleteVmOnFailMigration controls whether the target VM created by this Plan is deleted when a migration fails.
	// When true and the migration fails after the target VM has been created, the controller
	// will delete the target VM (and related target-side resources) during failed-migration cleanup
	// and when the Plan is deleted. When false (default), the target VM is preserved to aid
	// troubleshooting. The source VM is never modified.
	//
	// Note: If the Plan-level option is set to true, the VM-level option will be ignored.
	//
	// +optional
	DeleteVmOnFailMigration bool `json:"deleteVmOnFailMigration,omitempty"`
}

// Find a Hook for the specified step.
func (r *VM) FindHook(step string) (ref HookRef, found bool) {
	for _, h := range r.Hooks {
		if h.Step == step {
			found = true
			ref = h
			break
		}
	}

	return
}

// VM Status
type VMStatus struct {
	Timed `json:",inline"`
	VM    `json:",inline"`
	// Migration pipeline.
	Pipeline []*Step `json:"pipeline"`
	// Phase
	Phase string `json:"phase"`
	// Errors
	Error *Error `json:"error,omitempty"`
	// Warm migration status
	Warm *Warm `json:"warm,omitempty"`
	// Source VM power state before migration.
	RestorePowerState VMPowerState `json:"restorePowerState,omitempty"`
	// The firmware type detected from the OVF file produced by virt-v2v.
	Firmware string `json:"firmware,omitempty"`
	// The Operating System detected by virt-v2v.
	OperatingSystem string `json:"operatingSystem,omitempty"`
	// The new name of the VM after matching DNS1123 requirements.
	NewName string `json:"newName,omitempty"`

	// Conditions.
	libcnd.Conditions `json:",inline"`
}

// Warm Migration status
type Warm struct {
	Successes           int        `json:"successes"`
	Failures            int        `json:"failures"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	NextPrecopyAt       *meta.Time `json:"nextPrecopyAt,omitempty"`
	Precopies           []Precopy  `json:"precopies,omitempty"`
}

type VMPowerState string

const (
	VMPowerStateOn      VMPowerState = "On"
	VMPowerStateOff     VMPowerState = "Off"
	VMPowerStateUnknown VMPowerState = "Unknown"
)

// Precopy durations
type Precopy struct {
	Start        *meta.Time  `json:"start,omitempty"`
	End          *meta.Time  `json:"end,omitempty"`
	Snapshot     string      `json:"snapshot,omitempty"`
	CreateTaskId string      `json:"createTaskId,omitempty"`
	RemoveTaskId string      `json:"removeTaskId,omitempty"`
	Deltas       []DiskDelta `json:"deltas,omitempty"`
}

func (r *Precopy) WithDeltas(deltas map[string]string) {
	for disk, deltaId := range deltas {
		r.Deltas = append(r.Deltas, DiskDelta{Disk: disk, DeltaID: deltaId})
	}
}

func (r *Precopy) DeltaMap() map[string]string {
	mapping := make(map[string]string)
	for _, d := range r.Deltas {
		mapping[d.Disk] = d.DeltaID
	}
	return mapping
}

type DiskDelta struct {
	Disk    string `json:"disk"`
	DeltaID string `json:"deltaId"`
}

// Find a step by name.
func (r *VMStatus) FindStep(name string) (step *Step, found bool) {
	for _, s := range r.Pipeline {
		if s.Name == name {
			found = true
			step = s
			break
		}
	}

	return
}

// Add an error.
func (r *VMStatus) AddError(reason ...string) {
	if r.Error == nil {
		r.Error = &Error{Phase: r.Phase}
	}
	r.Error.Add(reason...)
}

// Reflect pipeline.
func (r *VMStatus) ReflectPipeline() {
	nStarted := 0
	nCompleted := 0
	for _, step := range r.Pipeline {
		if step.MarkedStarted() {
			nStarted++
		}
		if step.MarkedCompleted() {
			nCompleted++
		}
		if step.Error != nil {
			r.AddError(step.Error.Reasons...)
		}
	}
	if nStarted > 0 {
		r.MarkStarted()
	}
	if nCompleted == len(r.Pipeline) {
		r.MarkCompleted()
	}
}
