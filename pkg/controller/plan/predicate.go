package plan

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/handler"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	k8shandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PlanPredicate struct {
	predicate.Funcs
}

func (r PlanPredicate) Create(e event.CreateEvent) bool {
	_, cast := e.Object.(*api.Plan)
	if cast {
		libref.Mapper.Create(e)
		return true
	}

	return false
}

func (r PlanPredicate) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*api.Plan)
	if !cast {
		return false
	}
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(e)
	}

	return changed
}

func (r PlanPredicate) Delete(e event.DeleteEvent) bool {
	_, cast := e.Object.(*api.Plan)
	if cast {
		libref.Mapper.Delete(e)
		return true
	}

	return false
}

//
// Provider watch predicate.
// Also ensures an inventory watch is created and
// associated with the channel source.
type ProviderPredicate struct {
	handler.WatchManager
	predicate.Funcs
	channel chan event.GenericEvent
	client  client.Client
}

//
// Provider created event.
func (r *ProviderPredicate) Create(e event.CreateEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

//
// Provider updated event.
func (r *ProviderPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		if reconciled {
			r.ensureWatch(p)
			return true
		}
	}

	return false
}

//
// Provider deleted event.
func (r *ProviderPredicate) Delete(e event.DeleteEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

//
// Generic provider watch event.
func (r *ProviderPredicate) Generic(e event.GenericEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

//
// Ensure there is a watch for the provider
// and inventory API kinds.
func (r *ProviderPredicate) ensureWatch(p *api.Provider) {
	if !p.Status.HasCondition(libcnd.Ready) {
		return
	}
	h, err := handler.New(r.client, r.channel, p)
	if err != nil {
		log.Trace(err)
		return
	}
	err = h.Watch(&r.WatchManager)
	if err != nil {
		log.Trace(err)
		return
	}
}

type NetMapPredicate struct {
	predicate.Funcs
}

func (r NetMapPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (r NetMapPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.NetworkMap)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r NetMapPredicate) Delete(e event.DeleteEvent) bool {
	p, cast := e.Object.(*api.NetworkMap)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r NetMapPredicate) Generic(e event.GenericEvent) bool {
	p, cast := e.Object.(*api.NetworkMap)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

type DsMapPredicate struct {
	predicate.Funcs
}

func (r DsMapPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (r DsMapPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.StorageMap)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r DsMapPredicate) Delete(e event.DeleteEvent) bool {
	p, cast := e.Object.(*api.StorageMap)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r DsMapPredicate) Generic(e event.GenericEvent) bool {
	p, cast := e.Object.(*api.StorageMap)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

type HookPredicate struct {
	predicate.Funcs
}

func (r HookPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (r HookPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.Hook)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r HookPredicate) Delete(e event.DeleteEvent) bool {
	p, cast := e.Object.(*api.Hook)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r HookPredicate) Generic(e event.GenericEvent) bool {
	p, cast := e.Object.(*api.Hook)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

type MigrationPredicate struct {
	predicate.Funcs
}

func (r MigrationPredicate) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*api.Migration)
	if !cast {
		return false
	}
	pending := !object.Status.MarkedCompleted()
	return pending
}

func (r MigrationPredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*api.Migration)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*api.Migration)
	if !cast {
		return false
	}
	changed := old.Generation != new.Generation
	return changed
}

func (r MigrationPredicate) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*api.Migration)
	if !cast {
		return false
	}
	started := object.Status.MarkedStarted()
	return started
}

func (r MigrationPredicate) Generic(e event.GenericEvent) bool {
	return false
}

//
// Plan request for Migration.
func RequestForMigration(a k8shandler.MapObject) (list []reconcile.Request) {
	if m, cast := a.Object.(*api.Migration); cast {
		ref := &m.Spec.Plan
		if !libref.RefSet(ref) {
			return
		}
		list = append(
			list,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ref.Namespace,
					Name:      ref.Name,
				},
			})
	}

	return
}
