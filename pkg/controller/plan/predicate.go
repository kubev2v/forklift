package plan

import (
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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

type ProviderPredicate struct {
	predicate.Funcs
}

func (r ProviderPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (r ProviderPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r ProviderPredicate) Delete(e event.DeleteEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r ProviderPredicate) Generic(e event.GenericEvent) bool {
	p, cast := e.Object.(*api.Provider)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

type HostPredicate struct {
	predicate.Funcs
}

func (r HostPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (r HostPredicate) Update(e event.UpdateEvent) bool {
	p, cast := e.ObjectNew.(*api.Host)
	if cast {
		reconciled := p.Status.ObservedGeneration == p.Generation
		return reconciled
	}

	return false
}

func (r HostPredicate) Delete(e event.DeleteEvent) bool {
	return true
}

func (r HostPredicate) Generic(e event.GenericEvent) bool {
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
func RequestForMigration(a handler.MapObject) (list []reconcile.Request) {
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
