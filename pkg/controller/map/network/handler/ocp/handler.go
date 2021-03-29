package ocp

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

//
// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

//
// Ensure watch on networks.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	_, err = watch.Ensure(
		r.Provider(),
		&ocp.NetworkAttachmentDefinition{},
		r)

	return
}

//
// Resource created.
func (r *Handler) Created(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if network, cast := e.Resource.(*ocp.NetworkAttachmentDefinition); cast {
		r.changed(network)
	}
}

//
// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if !r.HasParity() {
		return
	}
	if network, cast := e.Resource.(*ocp.NetworkAttachmentDefinition); cast {
		r.changed(network)
	}
}

//
// Network changed.
// Find all of the NetworkMap CRs the reference both the
// provider and the changed network and enqueue reconcile events.
func (r *Handler) changed(network *ocp.NetworkAttachmentDefinition) {
	list := api.NetworkMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, mp := range list.Items {
		ref := mp.Spec.Provider.Destination
		if !r.MatchProvider(ref) {
			continue
		}
		for _, pair := range mp.Spec.Map {
			ref := pair.Destination
			if ref.Namespace == network.Namespace && ref.Name == network.Name {
				r.Enqueue(event.GenericEvent{
					Meta:   &mp.ObjectMeta,
					Object: &mp,
				})
				break
			}
		}
	}
}
