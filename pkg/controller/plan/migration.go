package plan

import (
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	"github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/builder"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
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
	PreHook  = "PreHook"
	PostHook = "PostHook"
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
	Started         = "Started"
	CreatePreHook   = "CreatePreHook"
	PreHookCreated  = "PreHookCreated"
	CreateImport    = "CreateImport"
	ImportCreated   = "ImportCreated"
	CreatePostHook  = "CreatePostHook"
	PostHookCreated = "PostHookCreated"
	Completed       = "Completed"
)

//
// Steps.
const (
	DiskTransfer    = "DiskTransfer"
	ImageConversion = "ImageConversion"
)

var (
	itinerary = libitr.Itinerary{
		Name: "",
		Pipeline: libitr.Pipeline{
			{Name: Started},
			{Name: CreatePreHook, All: HasPreHook},
			{Name: PreHookCreated, All: HasPreHook},
			{Name: CreateImport},
			{Name: ImportCreated},
			{Name: CreatePostHook, All: HasPostHook},
			{Name: PostHookCreated, All: HasPostHook},
			{Name: Completed},
		},
	}
)

//
// Migration.
type Migration struct {
	*plancontext.Context
	// Builder
	builder builder.Builder
	// kubevirt.
	kubevirt KubeVirt
	// VM import CRs.
	importMap ImportMap
}

//
// Type of migration.
func (r *Migration) Type() string {
	return r.Context.Source.Provider.Type()
}

//
// Run the migration.
func (r Migration) Run() (reQ time.Duration, err error) {
	reQ = PollReQ
	err = r.init()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	inFlight := 0
	list := r.Context.Plan.Status.Migration.VMs
	for _, vm := range list {
		if vm.MarkedCompleted() {
			continue
		}
		if inFlight > Settings.Migration.MaxInFlight {
			break
		}
		inFlight++
		itinerary.Predicate = &Predicate{
			vm: &vm.VM,
		}
		log.Info("Migration [RUN]:", "vm", vm)
		switch vm.Phase {
		case Started:
			vm.MarkStarted()
			vm.Phase = r.next(vm.Phase)
		case CreatePreHook:
			vm.Phase = r.next(vm.Phase)
		case PreHookCreated:
			vm.Phase = r.next(vm.Phase)
		case CreateImport:
			err = r.kubevirt.EnsureSecret(vm.Ref)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					vm.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
			err = r.kubevirt.EnsureImport(vm)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					vm.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
			err = r.kubevirt.SetSecretOwner(vm)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					vm.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
			vm.Phase = r.next(vm.Phase)
		case ImportCreated:
			completed, failed, rErr := r.updateVM(vm)
			if rErr != nil {
				err = liberr.Wrap(rErr)
				return
			}
			if completed {
				if !failed {
					vm.Phase = r.next(vm.Phase)
				} else {
					vm.Phase = Completed
				}
			}
		case CreatePostHook:
			vm.Phase = r.next(vm.Phase)
		case PostHookCreated:
			vm.Phase = r.next(vm.Phase)
		case Completed:
			vm.MarkCompleted()
			log.Info("Migration [COMPLETED]:", "vm", vm)
			inFlight--
		default:
			err = liberr.New("phase: unknown")
		}
		if vm.Error != nil {
			vm.Phase = Completed
		}
	}
	completed, err := r.end()
	if completed {
		reQ = NoReQ
	}

	return
}

//
// Cancel the migration.
// Delete resources associated with un-migrated VMs.
func (r *Migration) Cancel() (err error) {
	err = r.init()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, vm := range r.Plan.Status.Migration.VMs {
		if vm.MarkedCompleted() && vm.Error == nil {
			continue // migrated.
		}
		err = r.kubevirt.DeleteImport(vm)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

//
// Get/Build resources.
func (r *Migration) init() (err error) {
	r.builder, err = builder.New(r.Context)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.kubevirt = KubeVirt{
		Context: r.Context,
		Builder: r.builder,
	}

	return
}

//
// Next step in the itinerary.
func (r *Migration) next(phase string) (next string) {
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
func (r *Migration) begin() (err error) {
	if r.Plan.Status.HasAnyCondition(Executing, Succeeded, Failed) {
		return
	}
	r.Plan.Status.Migration.MarkStarted()
	r.Plan.Status.SetCondition(
		libcnd.Condition{
			Type:     Executing,
			Status:   True,
			Category: Advisory,
			Message:  "The plan is EXECUTING.",
			Durable:  true,
		})
	err = r.kubevirt.EnsureNamespace()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Delete
	for _, status := range r.Plan.Status.Migration.VMs {
		kept := []*plan.VMStatus{}
		if _, found := r.Plan.Spec.FindVM(status.ID); found {
			kept = append(kept, status)
		}
		r.Plan.Status.Migration.VMs = kept
	}
	//
	// Add/Update.
	list := []*plan.VMStatus{}
	for _, vm := range r.Plan.Spec.VMs {
		var status *plan.VMStatus
		itinerary.Predicate = &Predicate{vm: &vm}
		step, _ := itinerary.First()
		if current, found := r.Plan.Status.Migration.FindVM(vm.ID); !found {
			status = &plan.VMStatus{VM: vm}
		} else {
			status = current
		}
		if status.Phase != Completed || status.Error != nil {
			pipeline, pErr := r.buildPipeline(&vm)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			status.MarkReset()
			status.Pipeline = pipeline
			status.Phase = step.Name
			status.Error = nil
		}
		list = append(list, status)
	}

	r.Plan.Status.Migration.VMs = list

	log.Info("Execution [STARTED]:", "migration", r.Plan.Status.Migration)

	return
}

//
// Build the pipeline for a VM status.
func (r *Migration) buildPipeline(vm *plan.VM) (pipeline []*plan.Step, err error) {
	itinerary.Predicate = &Predicate{vm: vm}
	step, _ := itinerary.First()
	for {
		switch step.Name {
		case CreatePreHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PreHook,
						Description: "Run pre-migration hook.",
						Progress:    libitr.Progress{Total: 1},
					},
				})
		case CreateImport:
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
						Name:        DiskTransfer,
						Description: "Transfer disks.",
						Progress: libitr.Progress{
							Total: total,
						},
						Annotations: map[string]string{
							"unit": "MB",
						},
					},
					Tasks: tasks,
				})
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        ImageConversion,
						Description: "Convert image to kubevirt.",
						Progress:    libitr.Progress{Total: 1},
					},
				})
		case CreatePostHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PostHook,
						Description: "Run post-migration hook.",
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

	return
}

//
// End the migration.
func (r *Migration) end() (completed bool, err error) {
	failed := false
	for _, vm := range r.Plan.Status.Migration.VMs {
		if !vm.MarkedCompleted() {
			return
		}
		if vm.Error != nil {
			failed = true
			break
		}
	}
	r.Plan.Status.Migration.MarkCompleted()
	r.Plan.Status.DeleteCondition(Executing)
	if failed {
		log.Info("Execution [FAILED]")
		r.Plan.Status.SetCondition(
			libcnd.Condition{
				Type:     Failed,
				Status:   True,
				Category: Advisory,
				Message:  "The plan execution has FAILED.",
				Durable:  true,
			})
		err = r.Cancel()
		if err != nil {
			err = liberr.Wrap(err)
		}
	} else {
		log.Info("Execution [SUCCEEDED]")
		r.Plan.Status.SetCondition(
			libcnd.Condition{
				Type:     Succeeded,
				Status:   True,
				Category: Advisory,
				Message:  "The plan execution has SUCCEEDED.",
				Durable:  true,
			})
	}

	completed = true
	return
}

//
// Update VM migration status.
func (r *Migration) updateVM(vm *plan.VMStatus) (completed bool, failed bool, err error) {
	if r.importMap == nil {
		r.importMap, err = r.kubevirt.ImportMap()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	var imp VmImport
	found := false
	if imp, found = r.importMap[vm.ID]; !found {
		msg := "Import CR not found."
		vm.AddError(msg)
		return
	}
	r.updatePipeline(vm, &imp)
	vm.ReflectPipeline()
	conditions := imp.Conditions()
	cnd := conditions.FindCondition(Succeeded)
	if cnd != nil {
		vm.MarkedCompleted()
		completed = true
		if cnd.Status != True {
			vm.AddError(cnd.Message)
			failed = true
		}
	}

	return
}

//
// Update the pipeline.
func (r *Migration) updatePipeline(vm *plan.VMStatus, imp *VmImport) {
	for _, step := range vm.Pipeline {
		if step.MarkedCompleted() {
			continue
		}
		switch step.Name {
		case DiskTransfer:
			var name string
			var task *plan.Task
		nextDv:
			for _, dv := range imp.DataVolumes {
				switch r.Type() {
				case api.VSphere:
					name = dv.Spec.Source.VDDK.BackingFile
				default:
					continue nextDv
				}
				found := false
				task, found = step.FindTask(name)
				if !found {
					continue nextDv
				}
				conditions := dv.Conditions()
				cnd := conditions.FindCondition("Running")
				if cnd == nil {
					continue nextDv
				}
				task.MarkStarted()
				task.Phase = cnd.Reason
				pct := dv.PercentComplete()
				completed := pct * float64(task.Progress.Total)
				task.Progress.Completed = int64(completed)
				if conditions.HasCondition("Ready") {
					task.Progress.Completed = task.Progress.Total
					task.MarkCompleted()
				}
			}
		case ImageConversion:
			conditions := imp.Conditions()
			cnd := conditions.FindCondition("Processing")
			if cnd != nil {
				if cnd.Status == True && cnd.Reason == "ConvertingGuest" {
					step.MarkStarted()
				}
				if step.MarkedStarted() {
					step.Phase = cnd.Reason
				}
			}
			pct := imp.PercentComplete()
			completed := pct * float64(step.Progress.Total)
			step.Progress.Completed = int64(completed)
			cnd = conditions.FindCondition("Succeeded")
			if cnd != nil {
				step.MarkCompleted()
				step.Progress.Completed = step.Progress.Total
				if cnd.Status != True {
					step.AddError(cnd.Message)
					step.Phase = cnd.Reason
				}
			}
		}
		step.ReflectTasks()
		if step.Error != nil {
			vm.AddError(step.Error.Reasons...)
		}
	}
}

//
// Step predicate.
type Predicate struct {
	// VM listed on the plan.
	vm *plan.VM
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
