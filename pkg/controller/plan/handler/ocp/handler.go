package ocp

import (
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Package logger.
var log = logging.WithName("plan|ocp")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

// Ensure watch on VMs.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&ocp.VM{},
		r)
	if err != nil {
		return
	}

	log.Info(
		"Inventory watch ensured.",
		"provider",
		path.Join(
			r.Provider().Namespace,
			r.Provider().Name),
		"watch",
		w.ID())

	return
}

// Resource created.
func (r *Handler) Created(e libweb.Event) {
	if vm, cast := e.Resource.(*ocp.VM); cast {
		r.changed(vm)
	}
}

// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if vm, cast := e.Resource.(*ocp.VM); cast {
		r.changed(vm)
	}
}

// VM changed.
// Find all of the Plan CRs the reference both the provider
// and in the same target namespace and enqueue reconcile events.
func (r *Handler) changed(vm *ocp.VM) {
	log.V(3).Info(
		"VM changed.",
		"name",
		path.Join(
			vm.Namespace,
			vm.Name))
	list := api.PlanList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "failed to list Plan CRs")
		return
	}
	for i := range list.Items {
		plan := &list.Items[i]
		ref := plan.Spec.Provider.Destination
		if !r.MatchProvider(ref) {
			continue
		}
		if plan.Spec.TargetNamespace == vm.Namespace {
			log.V(3).Info(
				"Queue reconcile event.",
				"plan",
				path.Join(
					plan.Namespace,
					plan.Name))
			r.Enqueue(event.GenericEvent{
				Object: plan,
			})
		}
	}
}
