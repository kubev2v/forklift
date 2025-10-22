package vsphere

import (
	"path"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Package logger.
var log = logging.WithName("host|vsphere")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

// Ensure watch on hosts.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&vsphere.Host{},
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
	if host, cast := e.Resource.(*vsphere.Host); cast {
		r.changed(host)
	}
}

// Resource created.
func (r *Handler) Updated(e libweb.Event) {
	if host, cast := e.Resource.(*vsphere.Host); cast {
		updated := e.Updated.(*vsphere.Host)
		if updated.Path != host.Path || updated.InMaintenanceMode != host.InMaintenanceMode {
			r.changed(host, updated)
		}
	}
}

// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if host, cast := e.Resource.(*vsphere.Host); cast {
		r.changed(host)
	}
}

// Host changed.
// Find all of the HostMap CRs the reference both the
// provider and the changed host and enqueue reconcile events.
func (r *Handler) changed(models ...*vsphere.Host) {
	log.V(3).Info(
		"Host changed.",
		"id",
		models[0].ID)
	list := api.HostList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "failed to list Host CRs")
		return
	}
	for i := range list.Items {
		h := &list.Items[i]
		if !r.MatchProvider(h.Spec.Provider) {
			continue
		}
		referenced := false
		ref := h.Spec.Ref
		for _, host := range models {
			if ref.ID == host.ID || strings.HasSuffix(host.Path, ref.Name) {
				referenced = true
				break
			}
		}
		if referenced {
			log.V(3).Info(
				"Queue reconcile event.",
				"host",
				path.Join(
					h.Namespace,
					h.Name))
			r.Enqueue(event.GenericEvent{
				Object: h,
			})
		}
	}
}
