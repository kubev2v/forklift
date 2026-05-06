package conversion

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ConversionPredicate struct {
	predicate.TypedFuncs[*api.Conversion]
}

func (r ConversionPredicate) Create(e event.TypedCreateEvent[*api.Conversion]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r ConversionPredicate) Update(e event.TypedUpdateEvent[*api.Conversion]) bool {
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

func (r ConversionPredicate) Delete(e event.TypedDeleteEvent[*api.Conversion]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}
