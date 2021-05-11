package ovirt

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

//
// Handler factory.
func New(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h *Handler, err error) {
	//
	b, err := handler.New(client, channel, provider)
	if err != nil {
		return
	}
	h = &Handler{Handler: b}
	return
}
