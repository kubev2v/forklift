package openstack

import (
	"path"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Package logger.
var log = logging.WithName("storageMap|openstack")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

// Ensure watch on VolumeType.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&openstack.VolumeType{},
		r)
	if err != nil {
		return
	}
	log.Info(
		"VolumeType watch ensured.",
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
	if model, cast := e.Resource.(*openstack.VolumeType); cast {
		r.changed(model)
	}
}

// Resource updated.
func (r *Handler) Updated(e libweb.Event) {
	if model, cast := e.Resource.(*openstack.VolumeType); cast {
		updated := e.Updated.(*openstack.VolumeType)
		if updated.Path != model.Path {
			r.changed(model, updated)
		}
	}
}

// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if model, cast := e.Resource.(*openstack.VolumeType); cast {
		r.changed(model)
	}
}

// Storage changed.
// Find all StorageMap CRs that reference both the provider
// and the changed volume type, and enqueue reconcile events.
func (r *Handler) changed(models ...*openstack.VolumeType) {
	log.V(3).Info(
		"Volume type changed.",
		"id",
		models[0].ID)
	storageMapList := &api.StorageMapList{}
	err := r.List(context.TODO(), storageMapList)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "failed to list StorageMap CRs")
		return
	}
	for _, storageMap := range storageMapList.Items {
		ref := storageMap.Spec.Provider.Source
		if !r.MatchProvider(ref) {
			continue
		}

		if isReferenced(models, &storageMap) {
			log.V(3).Info(
				"Queue reconcile event.",
				"map",
				path.Join(
					storageMap.Namespace,
					storageMap.Name))
			r.Enqueue(event.GenericEvent{
				Object: &storageMap,
			})
		}
	}
}

func isReferenced(models []*openstack.VolumeType, storageMap *api.StorageMap) bool {
	for _, pair := range storageMap.Spec.Map {
		ref := pair.Source
		for _, model := range models {
			if ref.ID == model.ID || strings.HasSuffix(model.Path, ref.Name) {
				return true
			}
		}
	}
	return false
}
