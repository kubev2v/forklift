package handler

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Handler factory.
func New(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h *Handler, err error) {
	//
	h = &Handler{
		Client:   client,
		channel:  channel,
		provider: provider,
	}
	h.inventory, err = web.NewClient(provider)
	return
}
