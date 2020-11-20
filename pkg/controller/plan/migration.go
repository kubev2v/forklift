package plan

import (
	"context"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	"github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1/plan"
	"github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1/snapshot"
	"github.com/konveyor/virt-controller/pkg/controller/plan/builder"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	core "k8s.io/api/core/v1"
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
	// Migration
	Migration *api.Migration
	// The plan.
	Plan *api.Plan
	// Host client.
	Client client.Client
	// Source.
	source struct {
		// Provider
		provider *api.Provider
		// Secret.
		secret *core.Secret
	}
	// Destination.
	destination struct {
		// Provider.
		provider *api.Provider
		// k8s client.
		client client.Client
	}
	// Provider API client.
	inventory web.Client
	// Builder
	builder builder.Builder
	// kubevirt.
	kubevirt KubeVirt
	// VM import CRs.
	importMap ImportMap
	// Host map.
	hostMap map[string]*api.Host
}

//
// Type of migration.
func (r *Migration) Type() string {
	return r.source.provider.Type()
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
	list := r.Plan.Status.Migration.VMs
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
			err = r.kubevirt.EnsureSecret(vm.ID)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			err = r.kubevirt.EnsureImport(vm)
			if err != nil {
				err = liberr.Wrap(err)
				return
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
	if r.end() {
		reQ = NoReQ
	}

	return
}

//
// Get/Build resources.
func (r *Migration) init() (err error) {
	sn := snapshot.New(r.Migration)
	//
	// Source.
	r.source.provider = &api.Provider{}
	err = sn.Get(api.SourceSnapshot, r.source.provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	ref := r.source.provider.Spec.Secret
	r.source.secret = &core.Secret{}
	err = r.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		r.source.secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.inventory, err = web.NewClient(r.source.provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Destination.
	r.destination.provider = &api.Provider{}
	err = sn.Get(api.DestinationSnapshot, r.destination.provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !r.destination.provider.IsHost() {
		ref = r.destination.provider.Spec.Secret
		secret := &core.Secret{}
		err = r.Client.Get(
			context.TODO(),
			client.ObjectKey{
				Namespace: ref.Namespace,
				Name:      ref.Name,
			},
			secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.destination.client, err =
			r.destination.provider.Client(secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
		r.destination.client = r.Client
	}
	err = r.buildHostMap()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Builder & Reflector
	r.builder, err = builder.New(
		r.Client,
		r.inventory,
		r.source.provider,
		r.hostMap)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// kubevirt.
	r.kubevirt = KubeVirt{
		Builder:   r.builder,
		Secret:    r.source.secret,
		Client:    r.destination.client,
		Migration: r.Migration,
		Plan:      r.Plan,
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
			tasks, pErr := r.builder.Tasks(vm.ID)
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
func (r *Migration) end() (completed bool) {
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
// Build the host map (as needed).
func (r *Migration) buildHostMap() (err error) {
	list := &api.HostList{}
	err = r.Client.List(
		context.TODO(),
		&client.ListOptions{Namespace: r.Migration.Namespace},
		list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.hostMap = map[string]*api.Host{}
	for _, host := range list.Items {
		if !host.Status.HasCondition(libcnd.Ready) {
			continue
		}
		if r.source.provider.Namespace == host.Spec.Provider.Namespace &&
			r.source.provider.Name == host.Spec.Provider.Name {
			r.hostMap[host.Spec.ID] = &host
		}
	}

	return
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
