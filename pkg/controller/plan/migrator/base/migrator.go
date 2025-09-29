package base

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

type BaseMigrator struct {
	*plancontext.Context
	builder adapter.Builder
}

func (r *BaseMigrator) Init() (err error) {
	a, err := adapter.New(r.Context.Source.Provider)
	if err != nil {
		return
	}
	r.builder, err = a.Builder(r.Context)
	if err != nil {
		return
	}
	return
}

func (r *BaseMigrator) Logger() (logger logging.LevelLogger) {
	return r.Log
}

func (r *BaseMigrator) Begin() (err error) {
	return
}

func (r *BaseMigrator) Complete(vm *plan.VMStatus) {
}

func (r *BaseMigrator) Status(vm plan.VM) (status *plan.VMStatus) {
	if current, found := r.Context.Plan.Status.Migration.FindVM(vm.Ref); !found {
		status = &plan.VMStatus{VM: vm}
		if r.Context.Plan.Spec.Warm {
			status.Warm = &plan.Warm{}
		}
	} else {
		status = current
	}
	return
}

func (r *BaseMigrator) Reset(vm *plan.VMStatus, pipeline []*plan.Step) {
	vm.DeleteCondition(api.ConditionCanceled, api.ConditionFailed)
	vm.MarkReset()
	itr := r.Itinerary(vm.VM)
	step, _ := itr.First()
	vm.Phase = step.Name
	vm.Pipeline = pipeline
	vm.Error = nil
	if r.Context.Plan.Spec.Warm {
		vm.Warm = &plan.Warm{}
	}
}

func (r *BaseMigrator) Pipeline(vm plan.VM) (pipeline []*plan.Step, err error) {
	itinerary := r.Itinerary(vm)
	step, _ := itinerary.First()
	for {
		switch step.Name {
		case api.PhaseStarted:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        Initialize,
						Description: "Initialize migration.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       api.StepPending,
					},
				})
		case api.PhasePreHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        api.PhasePreHook,
						Description: "Run pre-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       api.StepPending,
					},
				})
		case api.PhaseAllocateDisks, api.PhaseCopyDisks, api.PhaseCopyDisksVirtV2V, api.PhaseConvertOpenstackSnapshot:
			tasks, pErr := r.builder.Tasks(vm.Ref)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			total := int64(0)
			for _, task := range tasks {
				total += task.Progress.Total
			}
			var taskDescription, taskName string
			switch step.Name {
			case api.PhaseCopyDisks:
				taskName = DiskTransfer
				taskDescription = "Transfer disks."
			case api.PhaseAllocateDisks:
				taskName = DiskAllocation
				taskDescription = "Allocate disks."
			case api.PhaseCopyDisksVirtV2V:
				taskName = DiskTransferV2v
				taskDescription = "Copy disks."
			case api.PhaseConvertOpenstackSnapshot:
				taskName = api.PhaseConvertOpenstackSnapshot
				taskDescription = "Convert OpenStack snapshot."
			default:
				err = liberr.New(fmt.Sprintf("Unknown step '%s'. Not implemented.", step.Name))
				return
			}
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        taskName,
						Description: taskDescription,
						Progress: libitr.Progress{
							Total: total,
						},
						Annotations: map[string]string{
							"unit": "MB",
						},
						Phase: api.StepPending,
					},
					Tasks: tasks,
				})
		case api.PhaseFinalize:
			tasks, pErr := r.builder.Tasks(vm.Ref)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			total := int64(0)
			for _, task := range tasks {
				total += task.Progress.Total
			}
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        Cutover,
						Description: "Finalize disk transfer.",
						Progress: libitr.Progress{
							Total: total,
						},
						Annotations: map[string]string{
							"unit": "MB",
						},
					},
					Tasks: tasks,
				})
		case api.PhaseConvertGuest:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        ImageConversion,
						Description: "Convert image to kubevirt.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       api.StepPending,
					},
				})
		case api.PhasePostHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        api.PhasePostHook,
						Description: "Run post-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       api.StepPending,
					},
				})
		case api.PhaseCreateVM:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        VMCreation,
						Description: "Create VM.",
						Phase:       api.StepPending,
						Progress:    libitr.Progress{Total: 1},
					},
				})
		case api.PhasePreflightInspection:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PreflightInspection,
						Description: "Inspect VM before migration.",
						Phase:       api.StepPending,
						Progress:    libitr.Progress{Total: 1},
					},
				})
		}
		next, done, _ := itinerary.Next(step.Name)
		if !done {
			step = next
		} else {
			break
		}
	}

	if len(pipeline) == 0 {
		err = liberr.New("Empty pipeline.", "vm", vm.String())
		return
	}

	r.Log.V(2).Info(
		"Pipeline built.",
		"vm",
		vm.String())
	return
}

func (r *BaseMigrator) Itinerary(vm plan.VM) (itinerary *libitr.Itinerary) {
	// Plan.Spec.Type supersedes the deprecated Warm boolean.
	if r.Context.Plan.Spec.Type == api.MigrationOnlyConversion {
		itinerary = r.onlyConversionItinerary()
	} else if r.Context.Plan.Spec.Warm {
		itinerary = r.warmItinerary()
	} else {
		itinerary = r.coldItinerary()
	}
	itinerary.Predicate = &BasePredicate{vm: &vm, context: r.Context}
	return
}

func (r *BaseMigrator) ExecutePhase(vm *plan.VMStatus) (ok bool, err error) {
	// return ok = false to delegate to default itinerary implementation
	// in plan/migration.go
	return
}

// Step gets the name of the pipeline step corresponding to the current VM phase.
func (r *BaseMigrator) Step(status *plan.VMStatus) (step string) {
	switch status.Phase {
	case api.PhaseStarted, api.PhaseCreateInitialSnapshot, api.PhaseWaitForInitialSnapshot, api.PhaseStoreInitialSnapshotDeltas:
		step = Initialize
	case api.PhaseAllocateDisks:
		step = DiskAllocation
	case api.PhaseCopyDisks, api.PhaseCopyingPaused, api.PhaseRemovePreviousSnapshot, api.PhaseWaitForPreviousSnapshotRemoval,
		api.PhaseCreateSnapshot, api.PhaseWaitForSnapshot, api.PhaseStoreSnapshotDeltas, api.PhaseAddCheckpoint,
		api.PhaseConvertOpenstackSnapshot, api.PhaseWaitForDataVolumesStatus:
		step = DiskTransfer
	case api.PhaseCreateDataVolumes:
		// This phase should be present in DiskTransfer step only when executing Preflight Inspection to avoid UI pipeline artifacts.
		// If not executing Preflight Inspection, keep the Initialize step.
		if r.Context.Plan.ShouldRunPreflightInspection() {
			step = DiskTransfer
		} else {
			step = Initialize
		}
	case api.PhaseRemovePenultimateSnapshot, api.PhaseWaitForPenultimateSnapshotRemoval, api.PhaseCreateFinalSnapshot,
		api.PhaseWaitForFinalSnapshot, api.PhaseAddFinalCheckpoint, api.PhaseFinalize, api.PhaseRemoveFinalSnapshot,
		api.PhaseWaitForFinalSnapshotRemoval, api.PhaseWaitForFinalDataVolumesStatus:
		step = Cutover
	case api.PhaseCreateGuestConversionPod, api.PhaseConvertGuest:
		step = ImageConversion
	case api.PhaseCopyDisksVirtV2V:
		step = DiskTransferV2v
	case api.PhaseCreateVM:
		step = VMCreation
	case api.PhasePreHook, api.PhasePostHook:
		step = status.Phase
	case api.PhaseStorePowerState, api.PhasePowerOffSource, api.PhaseWaitForPowerOff:
		if r.Context.Plan.Spec.Warm {
			step = Cutover
		} else {
			step = Initialize
		}
	case api.PhasePreflightInspection:
		step = PreflightInspection
	default:
		step = Unknown
	}
	return
}

func (r *BaseMigrator) warmItinerary() *libitr.Itinerary {
	return &libitr.Itinerary{
		Name: "Warm",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: HasPreHook},
			{Name: api.PhaseCreateInitialSnapshot},
			{Name: api.PhaseWaitForInitialSnapshot},
			{Name: api.PhaseStoreInitialSnapshotDeltas, All: VSphere},
			{Name: api.PhasePreflightInspection, All: RunInspection},
			{Name: api.PhaseCreateDataVolumes},
			// Precopy loop start
			{Name: api.PhaseWaitForDataVolumesStatus},
			{Name: api.PhaseCopyDisks},
			{Name: api.PhaseCopyingPaused},
			{Name: api.PhaseRemovePreviousSnapshot, All: VSphere},
			{Name: api.PhaseWaitForPreviousSnapshotRemoval, All: VSphere},
			{Name: api.PhaseCreateSnapshot},
			{Name: api.PhaseWaitForSnapshot},
			{Name: api.PhaseStoreSnapshotDeltas, All: VSphere},
			{Name: api.PhaseAddCheckpoint},
			// Precopy loop end
			{Name: api.PhaseStorePowerState},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: api.PhaseRemovePenultimateSnapshot, All: VSphere},
			{Name: api.PhaseWaitForPenultimateSnapshotRemoval, All: VSphere},
			{Name: api.PhaseCreateFinalSnapshot},
			{Name: api.PhaseWaitForFinalSnapshot},
			{Name: api.PhaseAddFinalCheckpoint},
			{Name: api.PhaseWaitForFinalDataVolumesStatus},
			{Name: api.PhaseFinalize},
			{Name: api.PhaseRemoveFinalSnapshot, All: VSphere},
			{Name: api.PhaseWaitForFinalSnapshotRemoval, All: VSphere},
			{Name: api.PhaseCreateGuestConversionPod, All: RequiresConversion},
			{Name: api.PhaseConvertGuest, All: RequiresConversion},
			{Name: api.PhaseCreateVM},
			{Name: api.PhasePostHook, All: HasPostHook},
			{Name: api.PhaseCompleted},
		},
	}
}

func (r *BaseMigrator) coldItinerary() *libitr.Itinerary {
	return &libitr.Itinerary{
		Name: "",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: HasPreHook},
			{Name: api.PhaseStorePowerState},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: api.PhaseCreateDataVolumes},
			{Name: api.PhaseCopyDisks, All: CDIDiskCopy},
			{Name: api.PhaseAllocateDisks, All: VirtV2vDiskCopy},
			{Name: api.PhaseCreateGuestConversionPod, All: RequiresConversion},
			{Name: api.PhaseConvertGuest, All: RequiresConversion},
			{Name: api.PhaseCopyDisksVirtV2V, All: RequiresConversion},
			{Name: api.PhaseConvertOpenstackSnapshot, All: OpenstackImageMigration},
			{Name: api.PhaseCreateVM},
			{Name: api.PhasePostHook, All: HasPostHook},
			{Name: api.PhaseCompleted},
		},
	}
}

func (r *BaseMigrator) onlyConversionItinerary() *libitr.Itinerary {
	return &libitr.Itinerary{
		Name: "OnlyConversion",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: HasPreHook},
			{Name: api.PhaseStorePowerState},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: api.PhaseCreateGuestConversionPod, All: RequiresConversion},
			{Name: api.PhaseConvertGuest, All: RequiresConversion},
			{Name: api.PhaseCreateVM},
			{Name: api.PhasePostHook, All: HasPostHook},
			{Name: api.PhaseCompleted},
		},
	}
}

// Step predicate.
type BasePredicate struct {
	// VM listed on the plan.
	vm *plan.VM
	// Plan context
	context *plancontext.Context
}

// Evaluate predicate flags.
func (r *BasePredicate) Evaluate(flag libitr.Flag) (allowed bool, err error) {
	useV2vForTransfer, vErr := r.context.Plan.ShouldUseV2vForTransfer()
	if vErr != nil {
		err = vErr
		return
	}

	switch flag {
	case HasPreHook:
		_, allowed = r.vm.FindHook(api.PhasePreHook)
	case HasPostHook:
		_, allowed = r.vm.FindHook(api.PhasePostHook)
	case RequiresConversion:
		allowed = r.context.Source.Provider.RequiresConversion() && !r.context.Plan.Spec.SkipGuestConversion
	case CDIDiskCopy:
		allowed = !useV2vForTransfer
	case VirtV2vDiskCopy:
		allowed = useV2vForTransfer
	case OpenstackImageMigration:
		allowed = r.context.Plan.IsSourceProviderOpenstack()
	case VSphere:
		allowed = r.context.Plan.IsSourceProviderVSphere()
	case RunInspection:
		allowed = r.context.Plan.ShouldRunPreflightInspection()
	}

	return
}

func (r *BasePredicate) Count() int {
	return 0x80
}
