package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
)

func (r *Migrator) Init() (err error) {
	r.log.V(1).Info("Initializing Azure migrator")
	return nil
}

func (r *Migrator) Begin() (err error) {
	r.log.V(1).Info("Azure migrator ready")
	return nil
}

func (r *Migrator) Complete(vm *planapi.VMStatus) {
	r.log.V(1).Info("Azure migration complete", "vm", vm.Name)
}

func (r *Migrator) Status(vm planapi.VM) *planapi.VMStatus {
	return &planapi.VMStatus{
		VM: vm,
	}
}

// Reset clears a VM's migration progress so it can be retried from the beginning.
func (r *Migrator) Reset(vm *planapi.VMStatus, pipeline []*planapi.Step) {
	vm.Pipeline = pipeline
	vm.Phase = api.PhaseStarted
	vm.Error = nil
	vm.Started = nil
	vm.Completed = nil

	r.log.V(1).Info("VM status reset", "vm", vm.Name)
}

func (r *Migrator) initialize(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Initializing Azure migration", "vm", vm.Name)

	vm.MarkStarted()

	if step, found := vm.FindStep(Initialize); found {
		step.MarkStarted()
		step.Phase = api.StepRunning
		step.Progress.Completed = 1
	}

	r.log.Info("Azure VM initialized successfully", "vm", vm.Name)

	r.NextPhase(vm)
	return true, nil
}
