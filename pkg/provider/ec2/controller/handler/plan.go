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

var log = logging.WithName("plan|ec2")

// PlanHandler handles VM inventory changes and triggers Plan reconciliation.
type PlanHandler struct {
	*handler.Handler
}

// Watch ensures periodic inventory events for plan reconciliation.
func (r *PlanHandler) Watch(watch *handler.WatchManager) (err error) {
	watch.EnsurePeriodicEvents(
		r.Provider(),
		&struct{}{},
		InventoryPollingInterval,
		r.generateEvents,
	)

	log.Info(
		"Periodic inventory events ensured.",
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
func (r *PlanHandler) Created(e libweb.Event) {
}

// Deleted is a no-op for EC2.
func (r *PlanHandler) Deleted(e libweb.Event) {
}

// generateEvents sends generic events for all plans.
func (r *PlanHandler) generateEvents() {
	list := api.PlanList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "Failed to list Plan CRs")
		return
	}

	for i := range list.Items {
		plan := &list.Items[i]
		if r.MatchProvider(plan.Spec.Provider.Source) || r.MatchProvider(plan.Spec.Provider.Destination) {
			r.Enqueue(event.GenericEvent{
				Object: plan,
			})
		}
	}
}
