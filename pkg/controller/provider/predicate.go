package provider

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ProviderPredicate struct {
	predicate.TypedFuncs[*api.Provider]
}

func (r ProviderPredicate) Create(e event.TypedCreateEvent[*api.Provider]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r ProviderPredicate) Update(e event.TypedUpdateEvent[*api.Provider]) bool {
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

func (r ProviderPredicate) Delete(e event.TypedDeleteEvent[*api.Provider]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}
