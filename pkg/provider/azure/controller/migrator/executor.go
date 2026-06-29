package migrator

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	migbase "github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// ExecutePhase runs the handler for the VM's current migration phase.
// Returns ok=false for phases handled by the base migrator (e.g. StorePowerState, Finalize).
func (r *Migrator) ExecutePhase(vm *planapi.VMStatus) (ok bool, err error) {
	r.log.V(1).Info("Executing Azure migration phase",
		"vm", vm.Name,
		"phase", vm.Phase)

	switch vm.Phase {
	case api.PhaseStarted:
		ok, err = r.initialize(vm)
	case api.PhasePreHook:
		ok = true
		r.NextPhase(vm)
	case PhaseCreatePreSnapshot:
		if step, found := vm.FindStep(PrepareSource); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok, err = r.createPreSnapshot(vm)
		if ok && err == nil {
			if step, found := vm.FindStep(PrepareSource); found {
				step.Progress.Completed = 1
			}
			r.NextPhase(vm)
		}
	case PhaseWaitForPreSnapshot:
		ok = true
		var ready bool
		ready, err = r.waitForPreSnapshot(vm)
		if err != nil {
			break
		}
		if ready {
			if step, found := vm.FindStep(PrepareSource); found {
				step.Progress.Completed = 2
			}
			r.NextPhase(vm)
		}
	case PhaseDeallocateVM:
		if step, found := vm.FindStep(PrepareSource); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok, err = r.deallocateVM(vm)
		if ok && err == nil {
			if step, found := vm.FindStep(PrepareSource); found {
				step.Progress.Completed = step.Progress.Total - 1
			}
			r.NextPhase(vm)
		}
	case PhaseWaitForDeallocation:
		ok = true
		var ready bool
		ready, err = r.waitForDeallocation(vm)
		if err != nil {
			break
		}
		if ready {
			if step, found := vm.FindStep(PrepareSource); found {
				step.Progress.Completed = step.Progress.Total
			}
			r.NextPhase(vm)
		}
	case PhaseCreateSnapshots:
		if step, found := vm.FindStep(CreateSnapshots); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok, err = r.createSnapshots(vm)
		if ok && err == nil {
			if step, found := vm.FindStep(CreateSnapshots); found {
				step.Progress.Completed = 1
			}
			r.NextPhase(vm)
		}
	case PhaseWaitForSnapshots:
		ok = true
		var ready bool
		ready, err = r.waitForSnapshots(vm)
		if err != nil {
			break
		}
		if ready {
			if step, found := vm.FindStep(CreateSnapshots); found {
				step.Progress.Completed = 2
			}
			r.NextPhase(vm)
		}
	case PhaseDeletePreSnapshots:
		ok, err = r.deletePreSnapshots(vm)
		if ok && err == nil {
			r.NextPhase(vm)
		}
	case PhaseCreateSnapshotContent:
		if step, found := vm.FindStep(DiskTransfer); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok, err = r.createSnapshotContent(vm)
		if ok && err == nil {
			r.NextPhase(vm)
		}
	case PhaseCreateVolumeSnapshot:
		ok, err = r.createVolumeSnapshot(vm)
		if ok && err == nil {
			r.NextPhase(vm)
		}
	case PhaseCreatePVCs:
		ok, err = r.createPVCs(vm)
		if ok && err == nil {
			r.NextPhase(vm)
		}
	case PhaseWaitForPVCsBound:
		ok = true
		var ready bool
		ready, err = r.waitForPVCsBound(vm)
		if err != nil {
			break
		}
		if ready {
			r.NextPhase(vm)
		}
	case PhaseInjectOwnerRefs:
		ok, err = r.injectOwnerRefs(vm)
		if ok && err == nil {
			if step, found := vm.FindStep(DiskTransfer); found {
				step.Progress.Completed = step.Progress.Total
			}
			r.NextPhase(vm)
		}
	case PhaseCopySnapshotsCrossRegion:
		if step, found := vm.FindStep(CreateSnapshots); found {
			step.Phase = api.StepRunning
		}
		ok, err = r.copySnapshotsCrossRegion(vm)
		if ok && err == nil {
			r.NextPhase(vm)
		}
	case PhaseWaitForCrossRegionSnapshots:
		ok = true
		var ready bool
		ready, err = r.waitForCrossRegionSnapshots(vm)
		if err != nil {
			break
		}
		if ready {
			if step, found := vm.FindStep(CreateSnapshots); found {
				step.Progress.Completed = step.Progress.Total
			}
			r.NextPhase(vm)
		}
	case api.PhaseStorePowerState:
		ok = false
	case api.PhaseCreateGuestConversionPod:
		ok = false
	case api.PhaseConvertGuest:
		ok = false
	case api.PhaseFinalize:
		ok = true
		r.NextPhase(vm)
	case api.PhaseCreateVM:
		ok = false
	case api.PhasePostHook:
		ok = true
		r.NextPhase(vm)
	case api.PhaseCompleted:
		ok = false
	default:
		err = liberr.New(fmt.Sprintf("Unknown phase: %s", vm.Phase))
	}

	return
}

func (r *Migrator) NextPhase(vm *planapi.VMStatus) {
	migbase.NextPhase(r, vm)

	if vm.Phase == api.PhaseCompleted {
		r.log.Info("Azure migration completed", "vm", vm.Name)
	} else {
		r.log.V(1).Info("Transitioned to next phase",
			"vm", vm.Name,
			"phase", vm.Phase)
	}
}

func (r *Migrator) StepError(vm *planapi.VMStatus, err error) {
	vm.AddError(err.Error())
	r.log.Error(err, "Migration step error",
		"vm", vm.Name,
		"phase", vm.Phase)
}

// Step maps a migration phase to its parent UI step name for progress reporting.
func (r *Migrator) Step(status *planapi.VMStatus) (step string) {
	switch status.Phase {
	case api.PhaseStarted:
		step = Initialize
	case api.PhasePreHook:
		step = api.PhasePreHook
	case api.PhaseStorePowerState, PhaseCreatePreSnapshot, PhaseWaitForPreSnapshot, PhaseDeallocateVM, PhaseWaitForDeallocation:
		step = PrepareSource
	case PhaseCreateSnapshots, PhaseWaitForSnapshots, PhaseDeletePreSnapshots, PhaseCopySnapshotsCrossRegion, PhaseWaitForCrossRegionSnapshots:
		step = CreateSnapshots
	case PhaseCreateSnapshotContent, PhaseCreateVolumeSnapshot, PhaseCreatePVCs, PhaseWaitForPVCsBound, PhaseInjectOwnerRefs:
		step = DiskTransfer
	case api.PhaseCreateGuestConversionPod, api.PhaseConvertGuest:
		step = api.PhaseConvertGuest
	case api.PhaseFinalize, api.PhaseCreateVM:
		step = CreateVM
	case api.PhasePostHook:
		step = api.PhasePostHook
	default:
		step = Initialize
	}
	return
}
