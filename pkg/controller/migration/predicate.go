package migration

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type MigrationPredicate struct {
	predicate.TypedFuncs[*api.Migration]
}

func (r MigrationPredicate) Create(e event.TypedCreateEvent[*api.Migration]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r MigrationPredicate) Update(e event.TypedUpdateEvent[*api.Migration]) bool {
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

func (r MigrationPredicate) Delete(e event.TypedDeleteEvent[*api.Migration]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}

type PlanPredicate struct {
	predicate.TypedFuncs[*api.Plan]
}

func (r PlanPredicate) Create(e event.TypedCreateEvent[*api.Plan]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

func (r PlanPredicate) Update(e event.TypedUpdateEvent[*api.Plan]) bool {
	p := e.ObjectNew
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}

func (r PlanPredicate) Delete(e event.TypedDeleteEvent[*api.Plan]) bool {
	return true
}

func (r PlanPredicate) Generic(e event.TypedGenericEvent[*api.Plan]) bool {
	p := e.Object
	reconciled := p.Status.ObservedGeneration == p.Generation
	return reconciled
}
