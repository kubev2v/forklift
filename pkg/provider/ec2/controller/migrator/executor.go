package migrator

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	migbase "github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// ExecutePhase executes a specific migration phase based on VM's current phase.
// Dispatches to appropriate handlers: power off, snapshot creation/waiting, data volume creation, cleanup.
// Returns ok=true when phase completes and VM should advance, false to retry same phase.
// Updates pipeline step status (running, completed) and progress tracking during execution.
func (r *Migrator) ExecutePhase(vm *planapi.VMStatus) (ok bool, err error) {
	r.log.V(1).Info("Executing EC2 migration phase",
		"vm", vm.Name,
		"phase", vm.Phase)

	switch vm.Phase {
	case api.PhaseStarted:
		ok, err = r.initialize(vm)
	case api.PhasePreHook:
		ok = true
		r.NextPhase(vm)
	case api.PhasePowerOffSource:
		if step, found := vm.FindStep(PrepareSource); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok, err = r.adpClient.PreTransferActions(vm.Ref)
		if ok && err == nil {
			if step, found := vm.FindStep(PrepareSource); found {
				step.Progress.Completed = 1
			}
			r.NextPhase(vm)
		}
	case api.PhaseWaitForPowerOff:
		ok, err = r.adpClient.PoweredOff(vm.Ref)
		if ok && err == nil {
			if step, found := vm.FindStep(PrepareSource); found {
				step.Progress.Completed = 2
			}
			r.NextPhase(vm)
		}
	case PhaseCreateSnapshots:
		ok, err = r.createSnapshots(vm)
		if ok && err == nil {
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
			r.NextPhase(vm)
		}
	case PhaseShareSnapshots:
		if step, found := vm.FindStep(ShareSnapshots); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok = true
		var ready bool
		ready, err = r.shareSnapshots(vm)
		if err != nil {
			break
		}
		if ready {
			if step, found := vm.FindStep(ShareSnapshots); found {
				step.Progress.Completed = 1
			}
			r.NextPhase(vm)
		}
	case PhaseCreateVolumes:
		if step, found := vm.FindStep(DiskTransfer); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok = true
		var ready bool
		ready, err = r.createVolumes(vm)
		if err != nil {
			break
		}
		if ready {
			r.NextPhase(vm)
		}
	case PhaseWaitForVolumes:
		ok = true
		var ready bool
		ready, err = r.waitForVolumes(vm)
		if err != nil {
			break
		}
		if ready {
			r.NextPhase(vm)
		}
	case PhaseCreatePVsAndPVCs:
		ok = true
		var ready bool
		ready, err = r.createPVsAndPVCs(vm)
		if err != nil {
			break
		}
		if ready {
			r.NextPhase(vm)
		}
	case api.PhaseCreateGuestConversionPod, api.PhaseConvertGuest:
		ok = false
	case api.PhaseFinalize:
		ok = false
	case api.PhaseCreateVM:
		ok = false
	case PhaseRemoveSnapshots:
		if step, found := vm.FindStep(Cleanup); found {
			if !step.MarkedStarted() {
				step.MarkStarted()
			}
			step.Phase = api.StepRunning
		}
		ok, err = r.removeSnapshots(vm)
		if ok && err == nil {
			r.NextPhase(vm)
		}
	case api.PhasePostHook:
		ok = true
		r.NextPhase(vm)
	case api.PhaseCompleted:
		ok = true
		vm.MarkCompleted()
		r.log.Info("EC2 migration completed", "vm", vm.Name)
	default:
		err = liberr.New(fmt.Sprintf("Unknown phase: %s", vm.Phase))
	}

	return
}

// NextPhase transitions VM to the next migration phase.
func (r *Migrator) NextPhase(vm *planapi.VMStatus) {
	migbase.NextPhase(r, vm)

	if vm.Phase == api.PhaseCompleted {
		r.log.Info("EC2 migration completed", "vm", vm.Name)
	} else {
		r.log.V(1).Info("Transitioned to next phase",
			"vm", vm.Name,
			"phase", vm.Phase)
	}
}

// StepError records migration step errors.
func (r *Migrator) StepError(vm *planapi.VMStatus, err error) {
	vm.AddError(err.Error())
	r.log.Error(err, "Migration step error",
		"vm", vm.Name,
		"phase", vm.Phase)
}

// Step maps VM phase to pipeline step name.
func (r *Migrator) Step(status *planapi.VMStatus) (step string) {
	switch status.Phase {
	case api.PhaseStarted:
		step = Initialize
	case api.PhasePreHook:
		step = api.PhasePreHook
	case api.PhasePowerOffSource, api.PhaseWaitForPowerOff:
		step = PrepareSource
	case PhaseCreateSnapshots, PhaseWaitForSnapshots:
		step = CreateSnapshots
	case PhaseShareSnapshots:
		step = ShareSnapshots
	case PhaseCreateVolumes, PhaseWaitForVolumes, PhaseCreatePVsAndPVCs:
		step = DiskTransfer
	case api.PhaseCreateGuestConversionPod, api.PhaseConvertGuest:
		step = ImageConversion
	case api.PhaseFinalize, api.PhaseCreateVM:
		step = CreateVM
	case PhaseRemoveSnapshots:
		step = Cleanup
	case api.PhasePostHook:
		step = api.PhasePostHook
	default:
		step = Initialize
	}
	return
}
