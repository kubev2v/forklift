package base

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Predicates.
var (
	HasPreHook              libitr.Flag = 0x01
	HasPostHook             libitr.Flag = 0x02
	RequiresConversion      libitr.Flag = 0x04
	CDIDiskCopy             libitr.Flag = 0x08
	VirtV2vDiskCopy         libitr.Flag = 0x10
	OpenstackImageMigration libitr.Flag = 0x20
	VSphere                 libitr.Flag = 0x40
	RunInspection           libitr.Flag = 0x80
)

// Steps.
const (
	Initialize          = "Initialize"
	Cutover             = "Cutover"
	DiskAllocation      = "DiskAllocation"
	DiskTransfer        = "DiskTransfer"
	ImageConversion     = "ImageConversion"
	DiskTransferV2v     = "DiskTransferV2v"
	VMCreation          = "VirtualMachineCreation"
	PreflightInspection = "PreflightInspection"
	Unknown             = "Unknown"
)

type Migrator interface {
	// Init the Migrator object.
	Init() error
	// Begin executing the migration plan.
	Begin() error
	// Complete cleans up after a VM migration is completed. This
	// must handle successful and unsuccessful completions, cancellations,
	// etc.
	Complete(*plan.VMStatus)
	// Status returns a VMStatus object for the VM.
	Status(plan.VM) *plan.VMStatus
	// Reset re-initializes a VM status and sets the pipeline.
	Reset(*plan.VMStatus, []*plan.Step)
	// Itinerary generates the itinerary for VM.
	Itinerary(plan.VM) *libitr.Itinerary
	// Pipeline generates the pipeline for a VM.
	Pipeline(plan.VM) ([]*plan.Step, error)
	// Step returns the name of the VM's current pipeline step.
	Step(*plan.VMStatus) string
	// ExecutePhase determines how to execute the VM's
	// current migration phase. If the migrator does not
	// implement the phase, it can return `false` to
	// delegate to the shared migration runner or else
	// return an error.
	ExecutePhase(*plan.VMStatus) (bool, error)
	// Logger must return a LevelLogger.
	Logger() logging.LevelLogger
}

// NextPhase transitions the VM to the next migration phase.
// If this was the last phase in the current pipeline step, the pipeline step
// is marked complete.
func NextPhase(migrator Migrator, vm *plan.VMStatus) {
	currentStep, found := vm.FindStep(migrator.Step(vm))
	if !found {
		vm.AddError(fmt.Sprintf("Step '%s' not found", migrator.Step(vm)))
		return
	}
	vm.Phase = next(migrator, vm)
	switch vm.Phase {
	case api.PhaseCompleted:
		// `Completed` is a terminal phase that does not belong
		// to a pipeline step. If it is the next VM phase, then
		// mark the current pipeline step complete without
		// looking for a following step.
		currentStep.MarkCompleted()
		currentStep.Phase = api.StepCompleted
	default:
		nextStep, found := vm.FindStep(migrator.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Next step '%s' not found", migrator.Step(vm)))
			return
		}
		if currentStep.Name != nextStep.Name {
			currentStep.MarkCompleted()
			currentStep.Phase = api.StepCompleted
			nextStep.MarkStarted()
			nextStep.Phase = api.StepRunning
		}
	}
}

// next determines the next phase the VM should move to.
func next(migrator Migrator, vm *plan.VMStatus) (next string) {
	itinerary := migrator.Itinerary(vm.VM)
	step, done, err := itinerary.Next(vm.Phase)
	if done || err != nil {
		next = api.PhaseCompleted
		if err != nil {
			migrator.Logger().Error(err, "Next phase failed.")
		}
	} else {
		next = step.Name
	}
	migrator.Logger().Info("Itinerary transition", "current phase", vm.Phase, "next phase", next)
	return
}
