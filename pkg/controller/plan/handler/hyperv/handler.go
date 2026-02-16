package hyperv

import (
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var log = logging.WithName("plan|hyperv")

type Handler struct {
	*handler.Handler
}

func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&hyperv.VM{},
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

func (r *Handler) Created(e libweb.Event) {
	if vm, cast := e.Resource.(*hyperv.VM); cast {
		r.changed(vm)
	}
}

func (r *Handler) Updated(e libweb.Event) {
	if vm, cast := e.Resource.(*hyperv.VM); cast {
		updated := e.Updated.(*hyperv.VM)
		if updated.Name != vm.Name {
			r.changed(vm, updated)
		}
	}
}

func (r *Handler) Deleted(e libweb.Event) {
	if vm, cast := e.Resource.(*hyperv.VM); cast {
		r.changed(vm)
	}
}

func (r *Handler) changed(models ...*hyperv.VM) {
	log.V(3).Info(
		"VM changed.",
		"id",
		models[0].ID)
	list := api.PlanList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "failed to list Plan CRs")
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
				if ref.ID == vm.ID || ref.Name == vm.Name {
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
