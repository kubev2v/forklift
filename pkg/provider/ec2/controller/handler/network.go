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

var logNetwork = logging.WithName("network|ec2")

// NetworkHandler handles network inventory changes and triggers NetworkMap reconciliation.
type NetworkHandler struct {
	*handler.Handler
}

// Watch ensures periodic inventory events for network mapping.
func (r *NetworkHandler) Watch(watch *handler.WatchManager) (err error) {
	watch.EnsurePeriodicEvents(
		r.Provider(),
		&struct{}{}, // Dummy type
		InventoryPollingInterval,
		r.generateEvents,
	)

	logNetwork.Info(
		"Periodic network mapping events ensured.",
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
func (r *NetworkHandler) Created(e libweb.Event) {
}

// Deleted is a no-op for EC2.
func (r *NetworkHandler) Deleted(e libweb.Event) {
}

// generateEvents sends generic events for all network mappings.
func (r *NetworkHandler) generateEvents() {
	list := api.NetworkMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		logNetwork.Error(err, "Failed to list NetworkMap CRs")
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
