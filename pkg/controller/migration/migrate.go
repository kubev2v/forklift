package migration

import (
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	"github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

//
// Requeue
const (
	NoReQ   = time.Duration(0)
	PollReQ = time.Second * 3
)

//
// Status pipeline/progress steps.
const (
	PreHook      = "PreHook"
	Import       = "Import"
	DiskTransfer = "DiskTransfer"
	PostHook     = "PostHook"
)

//
// Predicates.
var (
	HasPreHook  libitr.Flag = 0x01
	HasPostHook libitr.Flag = 0x02
)

//
// Phases.
const (
	Started           = "Started"
	CreatePreHook     = "CreatePreHook"
	PreHookCreated    = "PreHookCreated"
	PreHookSucceeded  = "PreHookSucceeded"
	PreHookFailed     = "PreHookFailed"
	CreateImport      = "CreateImport"
	ImportCreated     = "ImportCreated"
	ImportSucceeded   = "ImportSucceeded"
	ImportFailed      = "ImportFailed"
	CreatePostHook    = "CreatePostHook"
	PostHookCreated   = "PostHookCreated"
	PostHookSucceeded = "PostHookSucceeded"
	PostHookFailed    = "PostHookFailed"
	Completed         = "Completed"
)

var (
	itinerary = libitr.Itinerary{
		Name: "",
		Pipeline: libitr.Pipeline{
			{Name: Started},
			{Name: CreatePreHook, All: HasPreHook},
			{Name: PreHookCreated, All: HasPreHook},
			{Name: PreHookSucceeded, All: HasPreHook},
			{Name: CreateImport},
			{Name: ImportCreated},
			{Name: ImportSucceeded},
			{Name: CreatePostHook, All: HasPostHook},
			{Name: PostHookCreated, All: HasPostHook},
			{Name: PostHookSucceeded, All: HasPostHook},
			{Name: PostHookFailed, All: HasPostHook},
			{Name: Completed},
		},
	}
)

//
// Migration Task.
type Task struct {
	Client    client.Client
	Migration *api.Migration
	Plan      *api.Plan
}

//
// Run the migration.
func (r *Task) Run() (reQ time.Duration, err error) {
	reQ = PollReQ
	if r.Migration.HasCompleted() {
		reQ = NoReQ
		return
	}
	pErr := r.setPlan()
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}

	r.begin()

	for n := range r.Migration.Status.VMs {
		vm := &r.Migration.Status.VMs[n]
		if vm.Error != nil {
			continue
		}
		log.Info("Run:", "vm", vm.Planned.ID, "phase", vm.Phase)
		itinerary.Predicate = &Predicate{
			vm: &vm.Planned,
		}
		switch vm.Phase {
		case Started:
			vm.Phase = r.next(vm.Phase)
		case CreatePreHook:
			vm.Phase = r.next(vm.Phase)
		case PreHookCreated:
			vm.Phase = r.next(vm.Phase)
		case PreHookSucceeded:
			vm.Phase = r.next(vm.Phase)
		case PreHookFailed:
			vm.Error = &api.VMError{
				Phase:   vm.Phase,
				Reasons: []string{"This failed."},
			}
		case CreateImport:
			vm.Phase = r.next(vm.Phase)
		case ImportCreated:
			vm.Phase = r.next(vm.Phase)
		case ImportSucceeded:
			vm.Phase = r.next(vm.Phase)
		case ImportFailed:
			vm.Error = &api.VMError{
				Phase:   vm.Phase,
				Reasons: []string{"This failed."},
			}
		case CreatePostHook:
			vm.Phase = r.next(vm.Phase)
		case PostHookCreated:
			vm.Phase = r.next(vm.Phase)
		case PostHookSucceeded:
			vm.Phase = r.next(vm.Phase)
		case PostHookFailed:
			vm.Error = &api.VMError{
				Phase:   vm.Phase,
				Reasons: []string{"This failed."},
			}
		case Completed:
			reQ = NoReQ
			r.end()
		default:
			err = liberr.New("phase: unknown")
		}
		if vm.Error != nil {
			vm.Phase = Completed
		}
	}

	return
}

//
// Next step in the itinerary.
func (r *Task) next(phase string) (next string) {
	step, done, err := itinerary.Next(phase)
	if done || err != nil {
		next = Completed
		if err != nil {
			log.Trace(err)
		}
	} else {
		next = step.Name
	}

	return
}

//
// Begin the migration.
func (r *Task) begin() {
	if r.Migration.HasStarted() {
		return
	}
	now := meta.Now()
	r.Migration.Status.Started = &now
	r.Migration.Status.SetCondition(
		cnd.Condition{
			Type:     Running,
			Status:   True,
			Category: Advisory,
			Message:  "The migration is RUNNING.",
			Durable:  true,
		})
	list := []api.VMStatus{}
	for _, vm := range r.Plan.Spec.VMs {
		itinerary.Predicate = &Predicate{
			vm: &vm,
		}
		step, _ := itinerary.First()
		status := api.VMStatus{
			Planned:  vm,
			Pipeline: []api.Step{},
			Phase:    step.Name,
		}
		for {
			switch step.Name {
			case CreatePreHook:
				status.Pipeline = append(
					status.Pipeline,
					api.Step{
						Name:     PreHook,
						Progress: libitr.Progress{Total: 1},
					})
			case CreateImport:
				status.Pipeline = append(
					status.Pipeline,
					api.Step{
						Name:     DiskTransfer,
						Progress: libitr.Progress{Total: 1},
					})
				status.Pipeline = append(
					status.Pipeline,
					api.Step{
						Name:     Import,
						Progress: libitr.Progress{Total: 1},
					})
			case CreatePostHook:
				status.Pipeline = append(
					status.Pipeline,
					api.Step{
						Name:     PostHook,
						Progress: libitr.Progress{Total: 1},
					})
			}
			next, done, _ := itinerary.Next(step.Name)
			if !done {
				step = next
			} else {
				break
			}
		}

		list = append(list, status)
	}

	r.Migration.Status.VMs = list
}

//
// End the migration.
func (r *Task) end() {
	failed := false
	for _, vm := range r.Migration.Status.VMs {
		if vm.Error != nil {
			failed = true
			break
		}
	}
	now := meta.Now()
	r.Migration.Status.Completed = &now
	r.Migration.Status.DeleteCondition(Running)
	if failed {
		r.Migration.Status.SetCondition(
			cnd.Condition{
				Type:     Failed,
				Status:   True,
				Category: Advisory,
				Message:  "The migration has FAILED.",
				Durable:  true,
			})
	} else {
		r.Migration.Status.SetCondition(
			cnd.Condition{
				Type:     Succeeded,
				Status:   True,
				Category: Advisory,
				Message:  "The migration has SUCCEEDED.",
				Durable:  true,
			})
	}
}

//
// Get the associated plan.
func (r *Task) setPlan() (err error) {
	r.Plan = &api.Plan{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Migration.Spec.Plan.Namespace,
			Name:      r.Migration.Spec.Plan.Name,
		},
	}
	err = r.Migration.Snapshot().Get(r.Plan)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Step predicate.
type Predicate struct {
	// VM listed on the plan.
	vm *api.PlanVM
}

//
// Evaluate predicate flags.
func (r *Predicate) Evaluate(flag libitr.Flag) (allowed bool, err error) {
	if r.vm.Hook == nil {
		return
	}
	switch flag {
	case HasPreHook:
		allowed = ref.RefSet(r.vm.Hook.Before)
	case HasPostHook:
		allowed = ref.RefSet(r.vm.Hook.After)
	}

	return
}
