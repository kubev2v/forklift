package ocp

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"golang.org/x/net/context"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

//
// Package logger.
var log = logging.WithName("storageMap|ocp")

//
// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

//
// Ensure watch on storageClass.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&ocp.StorageClass{},
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
	if storageClass, cast := e.Resource.(*ocp.StorageClass); cast {
		r.changed(storageClass)
	}
}

//
// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if storageClass, cast := e.Resource.(*ocp.StorageClass); cast {
		r.changed(storageClass)
	}
}

//
// Storage changed.
// Find all of the StorageMap CRs the reference both the
// provider and the changed storageClass and enqueue reconcile events.
func (r *Handler) changed(storageClass *ocp.StorageClass) {
	log.V(3).Info(
		"StorageClass changed.",
		"name",
		storageClass.Name)
	list := api.StorageMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		mp := &list.Items[i]
		ref := mp.Spec.Provider.Destination
		if !r.MatchProvider(ref) {
			continue
		}
		for _, pair := range mp.Spec.Map {
			ref := pair.Destination
			if ref.StorageClass == storageClass.Name {
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
}
