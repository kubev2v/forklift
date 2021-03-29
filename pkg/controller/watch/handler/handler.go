package handler

import (
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

//
// Generic event.
type EventChannel chan event.GenericEvent

//
// Provider watch event handler.
type Handler struct {
	libweb.StockEventHandler
	// k8s client.
	client.Client
	// Event channel.
	channel EventChannel
	// Associated provider.
	provider *api.Provider
	// Inventory API client.
	inventory web.Client
	// Watch ended by peer.
	ended bool
	// Parity marker.
	parity bool
}

//
// The associated provider.
func (r *Handler) Provider() *api.Provider {
	return r.provider
}

//
// Get an inventory client.
func (r *Handler) Inventory() web.Client {
	return r.inventory
}

//
// Enqueue reconcile request.
func (r *Handler) Enqueue(event event.GenericEvent) {
	defer func() {
		recover()
	}()
	r.channel <- event
}

//
// Match provider.
func (r *Handler) MatchProvider(ref core.ObjectReference) bool {
	return r.Match(r.provider, ref)
}

//
// Ref matches object.
func (r *Handler) Match(object meta.Object, ref core.ObjectReference) bool {
	return ref.Namespace == object.GetNamespace() &&
		ref.Name == object.GetName()
}

//
// Inventory watch has parity.
func (r *Handler) HasParity() bool {
	return r.parity

}

//
// Inventory watch has parity.
func (r *Handler) Parity() {
	r.parity = true
}

//
// Watch ended by peer.
// The database has been closed.
func (r *Handler) End() {
	r.parity = false
	r.ended = true
}

//
// Watch error.
// Repair the watch.
func (r *Handler) Error(w *libweb.Watch, err error) {
	if !r.ended {
		_ = w.Repair()
	}
}
