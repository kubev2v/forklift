package openstack

import (
	"path"
	"strings"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Package logger.
var log = logging.WithName("storageMap|openstack")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

// Ensure watch on Images, Snapshots and Volumes.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&openstack.Image{},
		r)
	if err != nil {
		return
	}
	log.Info(
		"Image watch ensured.",
		"provider",
		path.Join(
			r.Provider().Namespace,
			r.Provider().Name),
		"watch",
		w.ID())

	w, err = watch.Ensure(
		r.Provider(),
		&openstack.Snapshot{},
		r)
	if err != nil {
		return
	}
	log.Info(
		"Snapshot watch ensured.",
		"provider",
		path.Join(
			r.Provider().Namespace,
			r.Provider().Name),
		"watch",
		w.ID())

	w, err = watch.Ensure(
		r.Provider(),
		&openstack.Volume{},
		r)
	if err != nil {
		return
	}
	log.Info(
		"Volume watch ensured.",
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
	r.changed(e.Resource)
}

// Resource updated.
func (r *Handler) Updated(e libweb.Event) {
	switch e.Resource.(type) {
	case *openstack.Image:
		image := e.Resource.(*openstack.Image)
		updated := e.Updated.(*openstack.Image)
		if updated.Path != image.Path {
			r.changed(image, updated)
		}
	case *openstack.Snapshot:
		snapshot := e.Resource.(*openstack.Snapshot)
		updated := e.Updated.(*openstack.Snapshot)
		if updated.Path != snapshot.Path {
			r.changed(snapshot, updated)
		}
	case *openstack.Volume:
		volume := e.Resource.(*openstack.Volume)
		updated := e.Updated.(*openstack.Volume)
		if updated.Path != volume.Path {
			r.changed(volume, updated)
		}
	}
}

// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	r.changed(e.Resource)
}

// Storage changed.
// Find all of the StorageMap CRs referencing both the
// provider and the changed storage and enqueue reconcile events.
func (r *Handler) changed(models ...interface{}) {

	list := api.StorageMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		storageMap := &list.Items[i]
		ref := storageMap.Spec.Provider.Source
		if !r.MatchProvider(ref) {
			continue
		}
		referenced := false
		for _, pair := range storageMap.Spec.Map {
			ref := pair.Source
			for _, model := range models {
				switch model.(type) {
				case *openstack.Image:
					image := model.(*openstack.Image)
					if ref.ID == image.ID || strings.HasSuffix(image.Path, ref.Name) {
						referenced = true
						log.V(3).Info(
							"Image changed.",
							"id",
							image.ID)
						break
					}
					return

				case *openstack.Snapshot:
					snapshot := model.(*openstack.Snapshot)
					if ref.ID == snapshot.ID || strings.HasSuffix(snapshot.Path, ref.Name) {
						referenced = true
						log.V(3).Info(
							"Snapshot changed.",
							"id",
							snapshot.ID)
						break
					}
					return

				case *openstack.Volume:
					volume := model.(*openstack.Volume)
					if ref.ID == volume.ID || strings.HasSuffix(volume.Path, ref.Name) {
						referenced = true
						log.V(3).Info(
							"Volume changed.",
							"id",
							volume.ID)
						break
					}
					return
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
					storageMap.Namespace,
					storageMap.Name))
			r.Enqueue(event.GenericEvent{
				Object: storageMap,
			})
		}
	}
}
