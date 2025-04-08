package base

import (
	"fmt"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
)

const Pending = "Pending"
const Canceled = "Canceled"
const Failed = "Failed"

// Package logger.
var log = logging.WithName("migrator|base")

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

func (r *BaseMigrator) Cleanup(status *plan.VMStatus, successful bool) (err error) {
	return
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

func (r *BaseMigrator) Reset(status *plan.VMStatus, pipeline []*plan.Step) {
	status.DeleteCondition(Canceled, Failed)
	status.MarkReset()
	itr := r.Itinerary(&BasePredicate{vm: &status.VM, context: r.Context})
	step, _ := itr.First()
	status.Phase = step.Name
	status.Pipeline = pipeline
	status.Error = nil
	if r.Context.Plan.Spec.Warm {
		status.Warm = &plan.Warm{}
	}
	return
}

func (r *BaseMigrator) Pipeline(vm plan.VM) (pipeline []*plan.Step, err error) {
	itinerary := r.Itinerary(&BasePredicate{vm: &vm, context: r.Context})
	step, _ := itinerary.First()
	for {
		switch step.Name {
		case Started:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        Initialize,
						Description: "Initialize migration.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case PreHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PreHook,
						Description: "Run pre-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case AllocateDisks, CopyDisks, CopyDisksVirtV2V, ConvertOpenstackSnapshot:
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
			case CopyDisks:
				taskName = DiskTransfer
				taskDescription = "Transfer disks."
			case AllocateDisks:
				taskName = DiskAllocation
				taskDescription = "Allocate disks."
			case CopyDisksVirtV2V:
				taskName = DiskTransferV2v
				taskDescription = "Copy disks."
			case ConvertOpenstackSnapshot:
				taskName = ConvertOpenstackSnapshot
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
						Phase: Pending,
					},
					Tasks: tasks,
				})
		case Finalize:
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
		case ConvertGuest:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        ImageConversion,
						Description: "Convert image to kubevirt.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case PostHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PostHook,
						Description: "Run post-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case CreateVM:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        VMCreation,
						Description: "Create VM.",
						Phase:       Pending,
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

func (r *BaseMigrator) Itinerary(predicate libitr.Predicate) (itinerary libitr.Itinerary) {
	if r.Context.Plan.Spec.Warm {
		itinerary = WarmItinerary
	} else {
		itinerary = ColdItinerary
	}
	itinerary.Predicate = predicate
	return
}

func (r *BaseMigrator) Next(status *plan.VMStatus) (next string) {
	itinerary := r.Itinerary(&BasePredicate{vm: &status.VM, context: r.Context})
	step, done, err := itinerary.Next(status.Phase)
	if done || err != nil {
		next = Completed
		if err != nil {
			log.Error(err, "Next phase failed.")
		}
	} else {
		next = step.Name
	}
	r.Log.Info("Itinerary transition", "current phase", status.Phase, "next phase", next)
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
	case Started, CreateInitialSnapshot, WaitForInitialSnapshot, StoreInitialSnapshotDeltas, CreateDataVolumes:
		step = Initialize
	case AllocateDisks:
		step = DiskAllocation
	case CopyDisks, CopyingPaused, RemovePreviousSnapshot, WaitForPreviousSnapshotRemoval, CreateSnapshot, WaitForSnapshot, StoreSnapshotDeltas, AddCheckpoint, ConvertOpenstackSnapshot, WaitForDataVolumesStatus:
		step = DiskTransfer
	case RemovePenultimateSnapshot, WaitForPenultimateSnapshotRemoval, CreateFinalSnapshot, WaitForFinalSnapshot, AddFinalCheckpoint, Finalize, RemoveFinalSnapshot, WaitForFinalSnapshotRemoval, WaitForFinalDataVolumesStatus:
		step = Cutover
	case CreateGuestConversionPod, ConvertGuest:
		step = ImageConversion
	case CopyDisksVirtV2V:
		step = DiskTransferV2v
	case CreateVM:
		step = VMCreation
	case PreHook, PostHook:
		step = status.Phase
	case StorePowerState, PowerOffSource, WaitForPowerOff:
		if r.Context.Plan.Spec.Warm {
			step = Cutover
		} else {
			step = Initialize
		}
	default:
		step = Unknown
	}
	return
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
		_, allowed = r.vm.FindHook(PreHook)
	case HasPostHook:
		_, allowed = r.vm.FindHook(PostHook)
	case RequiresConversion:
		allowed = r.context.Source.Provider.RequiresConversion()
	case CDIDiskCopy:
		allowed = !useV2vForTransfer
	case VirtV2vDiskCopy:
		allowed = useV2vForTransfer
	case OpenstackImageMigration:
		allowed = r.context.Plan.IsSourceProviderOpenstack()
	case VSphere:
		allowed = r.context.Plan.IsSourceProviderVSphere()
	}

	return
}

func (r *BasePredicate) Count() int {
	return 0x40
}
