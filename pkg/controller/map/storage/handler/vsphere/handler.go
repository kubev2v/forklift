package vsphere

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"golang.org/x/net/context"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"strings"
)

//
// Package logger.
var log = logging.WithName("storageMap|vsphere")

//
// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

//
// Ensure watch on Datastore.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&vsphere.Datastore{},
		r)

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

//
// Resource created.
func (r *Handler) Created(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if ds, cast := e.Resource.(*vsphere.Datastore); cast {
		r.changed(ds)
	}
}

//
// Resource created.
func (r *Handler) Updated(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if ds, cast := e.Resource.(*vsphere.Datastore); cast {
		updated := e.Updated.(*vsphere.Datastore)
		if updated.Path != ds.Path {
			r.changed(ds, updated)
		}
	}
}

//
// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if ds, cast := e.Resource.(*vsphere.Datastore); cast {
		r.changed(ds)
	}
}

//
// Storage changed.
// Find all of the StorageMap CRs the reference both the
// provider and the changed datastore and enqueue reconcile events.
func (r *Handler) changed(models ...*vsphere.Datastore) {
	log.V(3).Info(
		"Storage changed.",
		"id",
		models[0].ID)
	list := api.StorageMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		mp := &list.Items[i]
		ref := mp.Spec.Provider.Source
		if !r.MatchProvider(ref) {
			continue
		}
		referenced := false
		for _, pair := range mp.Spec.Map {
			ref := pair.Source
			for _, ds := range models {
				if ref.ID == ds.ID || strings.HasSuffix(ds.Path, ref.Name) {
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
				"map",
				path.Join(
					mp.Namespace,
					mp.Name))
			r.Enqueue(event.GenericEvent{
				Meta:   &mp.ObjectMeta,
				Object: mp,
			})
		}
	}
}
