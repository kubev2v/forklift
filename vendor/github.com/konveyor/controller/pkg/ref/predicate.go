package ref

import (
	"k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	Tag = "ref"
)

//
// Predicate Event Mapper
// All ObjectReference fields with the `ref` tag will be mapped.
//
// Example (CRD):
//     type Resource struct {
//         ThingRef *v1.ObjectReference `json:"thingRef" ref:"Thing"`
//     }
//
// Example (usage):
//     func (p Predicate) Create(e event.CreateEvent) bool {
//         ...
//         ref.Mapper.Create(e)
//     }}
//
type EventMapper struct {
	Map *RefMap
}

//
// Create event.
func (r *EventMapper) Create(event event.CreateEvent) {
	refOwner := Owner{
		Kind:      ToKind(event.Object),
		Namespace: event.Meta.GetNamespace(),
		Name:      event.Meta.GetName(),
	}
	for _, ref := range r.findRefs(event.Object) {
		r.Map.Add(refOwner, ref)
	}
}

//
// Update event.
func (r *EventMapper) Update(event event.UpdateEvent) {
	r.Map.DeleteOwner(Owner{
		Kind:      ToKind(event.ObjectOld),
		Namespace: event.MetaOld.GetNamespace(),
		Name:      event.MetaOld.GetName(),
	})
	refOwner := Owner{
		Kind:      ToKind(event.ObjectNew),
		Namespace: event.MetaNew.GetNamespace(),
		Name:      event.MetaNew.GetName(),
	}
	for _, ref := range r.findRefs(event.ObjectNew) {
		r.Map.Add(refOwner, ref)
	}
}

//
// Delete Mapper.
func (r *EventMapper) Delete(event event.DeleteEvent) {
	r.Map.DeleteOwner(Owner{
		Kind:      ToKind(event.Object),
		Namespace: event.Meta.GetNamespace(),
		Name:      event.Meta.GetName(),
	})
}

//
// Inspect the object for references.
func (r *EventMapper) findRefs(object interface{}) []Target {
	list := []Target{}
	rt := reflect.TypeOf(object)
	rv := reflect.ValueOf(object)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return list
	}
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i)
		fv := rv.Field(i)
		if kind, found := ft.Tag.Lookup(Tag); found {
			ref, cast := fv.Interface().(*v1.ObjectReference)
			if !cast || !RefSet(ref) {
				continue
			}
			list = append(
				list,
				Target{
					Kind:      kind,
					Namespace: ref.Namespace,
					Name:      ref.Name,
				})
		}
	}

	return list
}
