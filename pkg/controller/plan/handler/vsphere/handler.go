package vsphere

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"golang.org/x/net/context"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"strings"
)

// Package logger.
var log = logging.WithName("plan|vsphere")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

// Ensure watch on VMs.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&vsphere.VM{},
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
	if vm, cast := e.Resource.(*vsphere.VM); cast {
		r.changed(vm)
	}
}

// Resource created.
func (r *Handler) Updated(e libweb.Event) {
	if vm, cast := e.Resource.(*vsphere.VM); cast {
		updated := e.Updated.(*vsphere.VM)
		if updated.Path != vm.Path {
			r.changed(vm, updated)
		}
	}
}

// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if vm, cast := e.Resource.(*vsphere.VM); cast {
		r.changed(vm)
	}
}

// VM changed.
// Find all of the Plan CRs the reference both the
// provider and the changed VM and enqueue reconcile events.
func (r *Handler) changed(models ...*vsphere.VM) {
	log.V(3).Info(
		"VM changed.",
		"id",
		models[0].ID)
	list := api.PlanList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		plan := &list.Items[i]
		ref := plan.Spec.Provider.Source
		if plan.Spec.Archived || !r.MatchProvider(ref) {
			continue
		}
		referenced := false
		for _, planVM := range plan.Spec.VMs {
			ref := planVM.Ref
			for _, vm := range models {
				if ref.ID == vm.ID || strings.HasSuffix(vm.Path, ref.Name) {
					referenced = true
					break
				}
			}
			if referenced {
				break
			}
		}
		if referenced {
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
