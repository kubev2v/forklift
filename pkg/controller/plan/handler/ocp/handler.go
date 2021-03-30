package ocp

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

//
// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

//
// Ensure watch on VMs.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	_, err = watch.Ensure(
		r.Provider(),
		&ocp.VM{},
		r)

	return
}

//
// Resource created.
func (r *Handler) Created(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if vm, cast := e.Resource.(*ocp.VM); cast {
		r.changed(vm)
	}
}

//
// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if vm, cast := e.Resource.(*ocp.VM); cast {
		r.changed(vm)
	}
}

//
// VM changed.
// Find all of the Plan CRs the reference both the provider
// and in the same target namespace and enqueue reconcile events.
func (r *Handler) changed(vm *ocp.VM) {
	list := api.PlanList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, plan := range list.Items {
		ref := plan.Spec.Provider.Destination
		if !r.MatchProvider(ref) {
			continue
		}
		if plan.TargetNamespace() == vm.Namespace {
			r.Enqueue(event.GenericEvent{
				Meta:   &plan.ObjectMeta,
				Object: &plan,
			})
			break
		}
	}
}
