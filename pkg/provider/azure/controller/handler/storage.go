package handler

import (
	"context"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var logStorage = logging.WithName("storage|azure")

type StorageHandler struct {
	*handler.Handler
}

func (r *StorageHandler) Watch(watch *handler.WatchManager) (err error) {
	watch.EnsurePeriodicEvents(
		r.Provider(),
		&struct{}{},
		InventoryPollingInterval,
		r.generateEvents,
	)

	logStorage.Info(
		"Periodic storage mapping events ensured.",
		"provider",
		path.Join(
			r.Provider().Namespace,
			r.Provider().Name),
		"interval",
		InventoryPollingInterval,
	)

	return
}

func (r *StorageHandler) Created(e libweb.Event) {
}

func (r *StorageHandler) Deleted(e libweb.Event) {
}

func (r *StorageHandler) generateEvents() {
	list := api.StorageMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		logStorage.Error(err, "Failed to list StorageMap CRs")
		return
	}

	for i := range list.Items {
		mapping := &list.Items[i]
		if r.MatchProvider(mapping.Spec.Provider.Source) || r.MatchProvider(mapping.Spec.Provider.Destination) {
			r.Enqueue(event.GenericEvent{
				Object: mapping,
			})
		}
	}
}
