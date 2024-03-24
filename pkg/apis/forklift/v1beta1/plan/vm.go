package plan

import (
	"fmt"
	"path"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
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
	// Choose the primary disk the VM boots from
	// +optional
	RootDisk string `json:"rootDisk,omitempty"`
	// Selected InstanceType that will override the VM properties.
	// +optional
	InstanceType string `json:"instanceType,omitempty"`
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
	Start    *meta.Time `json:"start,omitempty"`
	End      *meta.Time `json:"end,omitempty"`
	Snapshot string     `json:"snapshot,omitempty"`
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
