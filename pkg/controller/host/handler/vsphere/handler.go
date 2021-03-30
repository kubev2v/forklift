package vsphere

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
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
// Ensure watch on hosts.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	_, err = watch.Ensure(
		r.Provider(),
		&vsphere.Host{},
		r)

	return
}

//
// Resource created.
func (r *Handler) Created(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if host, cast := e.Resource.(*vsphere.Host); cast {
		r.changed(host)
	}
}

//
// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if host, cast := e.Resource.(*vsphere.Host); cast {
		r.changed(host)
	}
}

//
// Host changed.
// Find all of the HostMap CRs the reference both the
// provider and the changed host and enqueue reconcile events.
func (r *Handler) changed(host *vsphere.Host) {
	list := api.HostList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, h := range list.Items {
		if !r.MatchProvider(h.Spec.Provider) {
			continue
		}
		inventory := r.Inventory()
		ref := h.Spec.Ref
		_, err = inventory.Host(&ref)
		if ref.ID == host.ID {
			r.Enqueue(event.GenericEvent{
				Meta:   &h.ObjectMeta,
				Object: &h,
			})
			break
		}
	}
}
