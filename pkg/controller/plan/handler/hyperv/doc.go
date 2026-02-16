package hyperv

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func New(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h *Handler, err error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	b, err := handler.New(client, channel, provider)
	if err != nil {
		return nil, fmt.Errorf("creating hyperv handler: %w", err)
	}
	h = &Handler{Handler: b}
	return
}
