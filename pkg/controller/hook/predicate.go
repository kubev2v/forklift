package hook

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type HookPredicate struct {
	predicate.TypedFuncs[*api.Hook]
}

func (r HookPredicate) Create(e event.TypedCreateEvent[*api.Hook]) bool {
	libref.Mapper.Create(event.CreateEvent{Object: e.Object})
	return true
}

func (r HookPredicate) Update(e event.TypedUpdateEvent[*api.Hook]) bool {
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

func (r HookPredicate) Delete(e event.TypedDeleteEvent[*api.Hook]) bool {
	libref.Mapper.Delete(event.DeleteEvent{Object: e.Object})
	return true
}
