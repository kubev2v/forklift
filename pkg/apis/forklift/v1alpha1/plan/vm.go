package plan

import (
	"fmt"
	libcnd "github.com/konveyor/controller/pkg/condition"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
)

//
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

//
// A VM listed on the plan.
type VM struct {
	ref.Ref `json:",inline"`
	// Enable hooks.
	Hooks []HookRef `json:"hooks,omitempty"`
}

//
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

//
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

	// Conditions.
	libcnd.Conditions `json:",inline"`
}

//
// Warm Migration status
type Warm struct {
	Successes           int        `json:"successes"`
	Failures            int        `json:"failures"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	NextPrecopyAt       *meta.Time `json:"nextPrecopyAt,omitempty"`
	Precopies           []Precopy  `json:"precopies,omitempty"`
}

// Precopy durations
type Precopy struct {
	Start *meta.Time `json:"start,omitempty"`
	End   *meta.Time `json:"end,omitempty"`
}

//
// Find the `Active` step.
func (r *VMStatus) ActiveStep() (step *Step, found bool) {
	for _, s := range r.Pipeline {
		if s.Name == r.Phase {
			found = true
			step = s
			break
		}
	}

	return
}

//
// Add an error.
func (r *VMStatus) AddError(reason ...string) {
	if r.Error == nil {
		r.Error = &Error{Phase: r.Phase}
	}
	r.Error.Add(reason...)
}

//
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
