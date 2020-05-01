package ref

import (
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

//
// Impl the handler interface.
func GetRequests(a handler.MapObject, source interface{}) []reconcile.Request {
	target := Target{
		Kind:      ToKind(a.Object),
		Name:      a.Meta.GetName(),
		Namespace: a.Meta.GetNamespace(),
	}
	list := []reconcile.Request{}
	for _, owner := range Map.Find(target) {
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
