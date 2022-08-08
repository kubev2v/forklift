package ref

import (
	"github.com/konveyor/controller/pkg/logging"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

//
// Build an event handler.
// Example:
//   err = cnt.Watch(
//      &source.Kind{
//         Type: &api.Referenced{},
//      },
//      libref.Handler(&api.Owner{}))
func Handler(owner interface{}) handler.EventHandler {
	log := logging.WithName("ref|handler")
	ownerKind := ToKind(owner)
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(
			func(a handler.MapObject) []reconcile.Request {
				refKind := ToKind(a.Object)
				list := GetRequests(ownerKind, a)
				if len(list) > 0 {
					log.V(4).Info(
						"handler: request list.",
						"referenced",
						refKind,
						"owner",
						ownerKind,
						"list",
						list)
				}
				return list
			}),
	}
}

//
// Impl the handler interface.
func GetRequests(kind string, a handler.MapObject) []reconcile.Request {
	target := Target{
		Kind:      ToKind(a.Object),
		Name:      a.Meta.GetName(),
		Namespace: a.Meta.GetNamespace(),
	}
	list := []reconcile.Request{}
	for _, owner := range Map.Find(target) {
		if owner.Kind != kind {
			continue
		}
		list = append(
			list,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: owner.Namespace,
					Name:      owner.Name,
				},
			})
	}

	return list
}

//
// Determine the resource Kind.
func ToKind(resource interface{}) string {
	t := reflect.TypeOf(resource).String()
	p := strings.SplitN(t, ".", 2)
	return string(p[len(p)-1])
}
