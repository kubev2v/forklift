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

var logNetwork = logging.WithName("network|azure")

type NetworkHandler struct {
	*handler.Handler
}

func (r *NetworkHandler) Watch(watch *handler.WatchManager) (err error) {
	watch.EnsurePeriodicEvents(
		r.Provider(),
		&struct{}{},
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

func (r *NetworkHandler) Created(e libweb.Event) {
}

func (r *NetworkHandler) Deleted(e libweb.Event) {
}

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
