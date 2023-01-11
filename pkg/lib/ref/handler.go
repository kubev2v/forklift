package ref

import (
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

// Build an event handler.
// Example:
//
//	err = cnt.Watch(
//	   &source.Kind{
//	      Type: &api.Referenced{},
//	   },
//	   libref.Handler(&api.Owner{}))
func Handler(owner interface{}) handler.EventHandler {
	log := logging.WithName("ref|handler")
	ownerKind := ToKind(owner)
	return handler.EnqueueRequestsFromMapFunc(
		func(a client.Object) []reconcile.Request {
			refKind := ToKind(a)
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
		})
}

// Impl the handler interface.
func GetRequests(kind string, a client.Object) []reconcile.Request {
	target := Target{
		Kind:      ToKind(a),
		Name:      a.GetName(),
		Namespace: a.GetNamespace(),
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

// Determine the resource Kind.
func ToKind(resource interface{}) string {
	t := reflect.TypeOf(resource).String()
	p := strings.SplitN(t, ".", 2)
	return string(p[len(p)-1])
}
