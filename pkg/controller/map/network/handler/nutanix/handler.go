package nutanix

import (
	"context"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var log = logging.WithName("networkMap|nutanix")

type Handler struct {
	*handler.Handler
}

func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&nutanix.Network{},
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

func (r *Handler) Created(e libweb.Event) {
	if network, cast := e.Resource.(*nutanix.Network); cast {
		r.changed(network)
	}
}

func (r *Handler) Updated(e libweb.Event) {
	if network, cast := e.Resource.(*nutanix.Network); cast {
		updated := e.Updated.(*nutanix.Network)
		if updated.Name != network.Name {
			r.changed(network, updated)
		}
	}
}

func (r *Handler) Deleted(e libweb.Event) {
	if network, cast := e.Resource.(*nutanix.Network); cast {
		r.changed(network)
	}
}

func referencesNetwork(mp *api.NetworkMap, models []*nutanix.Network) bool {
	for _, pair := range mp.Spec.Map {
		ref := pair.Source
		for _, network := range models {
			if ref.ID == network.ID || ref.Name == network.Name {
				return true
			}
		}
	}
	return false
}

func (r *Handler) changed(models ...*nutanix.Network) {
	log.V(3).Info(
		"Network changed.",
		"id",
		models[0].ID)
	list := api.NetworkMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "failed to list NetworkMap CRs")
		return
	}
	for i := range list.Items {
		mp := &list.Items[i]
		if !r.MatchProvider(mp.Spec.Provider.Source) {
			continue
		}
		if referencesNetwork(mp, models) {
			log.V(3).Info(
				"Queue reconcile event.",
				"map",
				path.Join(
					mp.Namespace,
					mp.Name))
			r.Enqueue(event.GenericEvent{
				Object: mp,
			})
		}
	}
}
