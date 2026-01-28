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

var logStorage = logging.WithName("storage|ec2")

// StorageHandler handles storage inventory changes and triggers StorageMap reconciliation.
type StorageHandler struct {
	*handler.Handler
}

// Watch ensures periodic inventory events for storage mapping.
func (r *StorageHandler) Watch(watch *handler.WatchManager) (err error) {
	watch.EnsurePeriodicEvents(
		r.Provider(),
		&struct{}{}, // Dummy type
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

// Created is a no-op for EC2.
func (r *StorageHandler) Created(e libweb.Event) {
}

// Deleted is a no-op for EC2.
func (r *StorageHandler) Deleted(e libweb.Event) {
}

// generateEvents sends generic events for all storage mappings.
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
