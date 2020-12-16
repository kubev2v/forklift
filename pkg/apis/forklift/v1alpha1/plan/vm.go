package plan

import "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"

//
// A VM listed on the plan.
type VM struct {
	ref.Ref `json:",inline"`
	// Enable hooks.
	Hook *Hook `json:"hook,omitempty"`
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
}

//
// Find a VM status.
func (r *MigrationStatus) FindVM(vmID string) (v *VMStatus, found bool) {
	for _, vm := range r.VMs {
		if vm.ID == vmID {
			found = true
			v = vm
			return
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
			step.AddError(step.Error.Reasons...)
		}
	}
	if nStarted > 0 {
		r.MarkedStarted()
	}
	if nCompleted == len(r.Pipeline) {
		r.MarkCompleted()
	}
}
