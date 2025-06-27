package plan

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/plan/handler"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PlanPredicate struct {
	predicate.TypedFuncs[*api.Plan]
}

func (r PlanPredicate) Create(e event.TypedCreateEvent[*api.Plan]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r PlanPredicate) Update(e event.TypedUpdateEvent[*api.Plan]) bool {
	object := e.ObjectNew
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(event.UpdateEvent{
			ObjectOld: e.ObjectOld,
			ObjectNew: e.ObjectNew,
		})
	}

	return changed
}

func (r PlanPredicate) Delete(e event.TypedDeleteEvent[*api.Plan]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}

// Provider watch predicate.
// Also ensures an inventory watch is created and
// associated with the channel source.
type ProviderPredicate struct {
	handler.WatchManager
	predicate.TypedFuncs[*api.Provider]
	channel chan event.GenericEvent
	client  client.Client
}

// Provider created event.
func (r *ProviderPredicate) Create(e event.TypedCreateEvent[*api.Provider]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

// Provider updated event.
func (r *ProviderPredicate) Update(e event.TypedUpdateEvent[*api.Provider]) bool {
	p := e.ObjectNew
	reconciled := p.Status.ObservedGeneration == p.Generation
	if reconciled {
		r.ensureWatch(p)
		return true
	}

	return false
}

// Provider deleted event.
func (r *ProviderPredicate) Delete(e event.TypedDeleteEvent[*api.Provider]) bool {
	p := e.Object
	r.WatchManager.Deleted(p)
	return true
}

// Generic provider watch event.
func (r *ProviderPredicate) Generic(e event.TypedGenericEvent[*api.Provider]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

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
	predicate.TypedFuncs[*api.NetworkMap]
}

func (r NetMapPredicate) Create(e event.TypedCreateEvent[*api.NetworkMap]) bool {
	return false
}

func (r NetMapPredicate) Update(e event.TypedUpdateEvent[*api.NetworkMap]) bool {
	p := e.ObjectNew
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

func (r NetMapPredicate) Delete(e event.TypedDeleteEvent[*api.NetworkMap]) bool {
	return true
}

func (r NetMapPredicate) Generic(e event.TypedGenericEvent[*api.NetworkMap]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

type DsMapPredicate struct {
	predicate.TypedFuncs[*api.StorageMap]
}

func (r DsMapPredicate) Create(e event.TypedCreateEvent[*api.StorageMap]) bool {
	return false
}

func (r DsMapPredicate) Update(e event.TypedUpdateEvent[*api.StorageMap]) bool {
	p := e.ObjectNew
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

func (r DsMapPredicate) Delete(e event.TypedDeleteEvent[*api.StorageMap]) bool {
	return true
}

func (r DsMapPredicate) Generic(e event.TypedGenericEvent[*api.StorageMap]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

type HookPredicate struct {
	predicate.TypedFuncs[*api.Hook]
}

func (r HookPredicate) Create(e event.TypedCreateEvent[*api.Hook]) bool {
	return false
}

func (r HookPredicate) Update(e event.TypedUpdateEvent[*api.Hook]) bool {
	p := e.ObjectNew
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

func (r HookPredicate) Delete(e event.TypedDeleteEvent[*api.Hook]) bool {
	return true
}

func (r HookPredicate) Generic(e event.TypedGenericEvent[*api.Hook]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

type MigrationPredicate struct {
	predicate.TypedFuncs[*api.Migration]
}

func (r MigrationPredicate) Create(e event.TypedCreateEvent[*api.Migration]) bool {
	object := e.Object
	pending := !object.Status.MarkedCompleted()
	return pending
}

func (r MigrationPredicate) Update(e event.TypedUpdateEvent[*api.Migration]) bool {
	old := e.ObjectOld
	new := e.ObjectNew
	changed := old.Generation != new.Generation
	return changed
}

func (r MigrationPredicate) Delete(e event.TypedDeleteEvent[*api.Migration]) bool {
	object := e.Object
	started := object.Status.MarkedStarted()
	return started
}

func (r MigrationPredicate) Generic(e event.TypedGenericEvent[*api.Migration]) bool {
	return false
}

// Plan request for Migration.
func RequestForMigration(ctx context.Context, a client.Object) (list []reconcile.Request) {
	if m, cast := a.(*api.Migration); cast {
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
