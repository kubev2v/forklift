package hook

import (
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type HookPredicate struct {
	predicate.Funcs
}

func (r HookPredicate) Create(e event.CreateEvent) bool {
	_, cast := e.Object.(*api.Hook)
	if cast {
		libref.Mapper.Create(e)
		return true
	}

	return false
}

func (r HookPredicate) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*api.Hook)
	if !cast {
		return false
	}
	changed := object.Status.ObservedGeneration < object.Generation
	if changed {
		libref.Mapper.Update(e)
	}

	return changed
}

func (r HookPredicate) Delete(e event.DeleteEvent) bool {
	_, cast := e.Object.(*api.Hook)
	if cast {
		libref.Mapper.Delete(e)
		return true
	}

	return false
}
