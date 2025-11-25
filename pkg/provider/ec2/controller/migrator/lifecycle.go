package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
)

// Init initializes the EC2 migrator at plan level before VMs begin migration.
// No-op for EC2 since client connection and credential validation happen in New().
func (r *Migrator) Init() (err error) {
	r.log.V(1).Info("Initializing EC2 migrator")
	return nil
}

// Begin prepares the migrator to start processing VMs.
// No-op for EC2 since all components are initialized and AWS connectivity is established.
func (r *Migrator) Begin() (err error) {
	r.log.V(1).Info("EC2 migrator ready")
	return nil
}

// Complete performs final cleanup after VM migration finishes.
// Minimal for EC2 since cleanup (snapshot deletion, secret removal) is handled by RemoveSnapshots phase.
func (r *Migrator) Complete(vm *planapi.VMStatus) {
	r.log.V(1).Info("EC2 migration complete", "vm", vm.Name)
}

// Status creates a new VMStatus object for tracking migration progress.
// Returns a VMStatus initialized with the VM definition, ready to be populated with pipeline and progress.
func (r *Migrator) Status(vm planapi.VM) *planapi.VMStatus {
	return &planapi.VMStatus{
		VM: vm,
	}
}

// Reset re-initializes a VM's migration status for retry after failure or cancellation.
// Replaces pipeline, resets phase to Started, clears errors and timestamps. Preserves VM reference.
func (r *Migrator) Reset(vm *planapi.VMStatus, pipeline []*planapi.Step) {
	vm.Pipeline = pipeline
	vm.Phase = api.PhaseStarted
	vm.Error = nil
	vm.Started = nil
	vm.Completed = nil

	r.log.V(1).Info("VM status reset", "vm", vm.Name)
}

// initialize starts the migration workflow, marking VM as started and updating the Initialize step.
// First phase that runs when VM migration begins. Records start timestamp and advances to next phase.
// No EC2-specific work needed - validation/connection already done.
func (r *Migrator) initialize(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Initializing EC2 migration", "vm", vm.Name)

	vm.MarkStarted()

	if step, found := vm.FindStep(Initialize); found {
		step.MarkStarted()
		step.Phase = api.StepRunning
		step.Progress.Completed = 1
	}

	r.log.Info("EC2 VM initialized successfully", "vm", vm.Name)

	r.NextPhase(vm)
	return true, nil
}
