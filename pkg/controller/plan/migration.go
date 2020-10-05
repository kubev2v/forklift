package plan

import (
	"context"
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	"github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	kubevirt "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	core "k8s.io/api/core/v1"
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
	Started         = "Started"
	CreatePreHook   = "CreatePreHook"
	PreHookCreated  = "PreHookCreated"
	CreateImport    = "CreateImport"
	ImportCreated   = "ImportCreated"
	CreatePostHook  = "CreatePostHook"
	PostHookCreated = "PostHookCreated"
	Completed       = "Completed"
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
	// Host client.
	Client client.Client
	// The plan.
	Plan *api.Plan
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
		// Secret.
		secret *core.Secret
		// k8s client.
		client client.Client
	}
	// kubevirt.
	kubevirt KubeVirt
	// VM import CRs.
	importMap ImportMap
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
	for n := range list {
		vm := &list[n]
		if vm.Done() {
			continue
		}
		if inFlight > Settings.Migration.MaxInFlight {
			break
		}
		inFlight++
		log.Info("Migrate:", "vm", vm.Planned.ID, "phase", vm.Phase)
		itinerary.Predicate = &Predicate{
			vm: &vm.Planned,
		}
		switch vm.Phase {
		case Started:
			now := meta.Now()
			vm.Started = &now
			vm.Phase = r.next(vm.Phase)
		case CreatePreHook:
			vm.Phase = r.next(vm.Phase)
		case PreHookCreated:
			vm.Phase = r.next(vm.Phase)
		case CreateImport:
			err = r.kubevirt.CreateImport(vm.Planned.ID)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			vm.Phase = r.next(vm.Phase)
		case ImportCreated:
			r.reflectImport(vm)
		case CreatePostHook:
			vm.Phase = r.next(vm.Phase)
		case PostHookCreated:
			vm.Phase = r.next(vm.Phase)
		case Completed:
			inFlight--
			if vm.Completed == nil {
				now := meta.Now()
				vm.Completed = &now
			}
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
	//
	// Source.
	r.source.provider = r.Plan.Status.Migration.GetSource()
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
	}
	//
	// Destination.
	r.destination.provider = r.Plan.Status.Migration.GetDestination()
	ref = r.destination.provider.Spec.Secret
	r.destination.secret = &core.Secret{}
	err = r.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		r.destination.secret)
	if err != nil {
		err = liberr.Wrap(err)
	}
	r.destination.client, err =
		r.destination.provider.Client(r.destination.secret)
	//
	// kubevirt.
	r.kubevirt = KubeVirt{
		Plan: r.Plan,
	}
	pClient, err := web.NewClient(r.source.provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.kubevirt.Source.Provider = r.source.provider
	r.kubevirt.Source.Secret = r.source.secret
	r.kubevirt.Source.Client = pClient
	r.kubevirt.Destination.Provider = r.destination.provider
	r.kubevirt.Destination.Client = r.destination.client
	//
	// Import Map
	r.importMap, err = r.kubevirt.ListImports()
	if err != nil {
		err = liberr.Wrap(err)
		return
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
	now := meta.Now()
	r.Plan.Status.Migration.Started = &now
	r.Plan.Status.Migration.Completed = nil
	r.Plan.Status.SetCondition(
		cnd.Condition{
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
	err = r.kubevirt.EnsureSecret()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.kubevirt.EnsureMapping()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Delete
	for _, status := range r.Plan.Status.Migration.VMs {
		kept := []api.VMStatus{}
		if _, found := r.Plan.Spec.FindVM(status.Planned.ID); found {
			kept = append(kept, status)
		}
		r.Plan.Status.Migration.VMs = kept
	}
	//
	// Add/Update.
	list := []api.VMStatus{}
	for _, vm := range r.Plan.Spec.VMs {
		var status api.VMStatus
		itinerary.Predicate = &Predicate{vm: &vm}
		step, _ := itinerary.First()
		if current, found := r.Plan.Status.Migration.FindVM(vm.ID); !found {
			status = api.VMStatus{Planned: vm}
		} else {
			status = *current
		}
		if status.Phase != Completed || status.Error != nil {
			status.Started = nil
			status.Completed = nil
			status.Pipeline = r.buildPipeline(&vm)
			status.Phase = step.Name
			status.Error = nil
		}
		list = append(list, status)
	}

	r.Plan.Status.Migration.VMs = list

	return
}

//
// Build the pipeline for a VM status.
func (r *Migration) buildPipeline(vm *api.PlanVM) (pipeline []api.Step) {
	itinerary.Predicate = &Predicate{vm: vm}
	step, _ := itinerary.First()
	for {
		switch step.Name {
		case CreatePreHook:
			pipeline = append(
				pipeline,
				api.Step{
					Name:     PreHook,
					Progress: libitr.Progress{Total: 1},
				})
		case CreateImport:
			pipeline = append(
				pipeline,
				api.Step{
					Name:     DiskTransfer,
					Progress: libitr.Progress{Total: 1},
				})
			pipeline = append(
				pipeline,
				api.Step{
					Name:     Import,
					Progress: libitr.Progress{Total: 1},
				})
		case CreatePostHook:
			pipeline = append(
				pipeline,
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

	return
}

//
// End the migration.
func (r *Migration) end() (completed bool) {
	failed := false
	for _, vm := range r.Plan.Status.Migration.VMs {
		if !vm.Done() {
			return
		}
		if vm.Error != nil {
			failed = true
			break
		}
	}
	now := meta.Now()
	r.Plan.Status.Migration.Completed = &now
	r.Plan.Status.DeleteCondition(Executing)
	if failed {
		r.Plan.Status.SetCondition(
			cnd.Condition{
				Type:     Failed,
				Status:   True,
				Category: Advisory,
				Message:  "The plan execution has FAILED.",
				Durable:  true,
			})
	} else {
		r.Plan.Status.SetCondition(
			cnd.Condition{
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
// Apply VM Import status.
func (r *Migration) reflectImport(vm *api.VMStatus) {
	var _import *kubevirt.VirtualMachineImport
	found := false
	addErr := func(reason *string) {
		if reason == nil {
			return
		}
		if vm.Error == nil {
			vm.Error = &api.VMError{
				Reasons: []string{*reason},
				Phase:   vm.Phase,
			}
		} else {
			vm.Error.Reasons = append(vm.Error.Reasons, *reason)
		}
	}
	if _import, found = r.importMap[vm.Planned.ID]; !found {
		msg := "Import CR not found."
		addErr(&msg)
		return
	}
	for _, condition := range _import.Status.Conditions {
		switch condition.Type {
		case "Succeeded":
			if condition.Status == False {
				addErr(condition.Message)
			}
		case "Valid":
			if condition.Status == False {
				addErr(condition.Message)
			}
		}
	}
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
