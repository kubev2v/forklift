package handler

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func New(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h *PlanHandler, err error) {
	b, err := handler.New(client, channel, provider)
	if err != nil {
		return
	}
	h = &PlanHandler{Handler: b}
	return
}

func NewNetworkHandler(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h *NetworkHandler, err error) {
	b, err := handler.New(client, channel, provider)
	if err != nil {
		return
	}
	h = &NetworkHandler{Handler: b}
	return
}

func NewStorageHandler(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h *StorageHandler, err error) {
	b, err := handler.New(client, channel, provider)
	if err != nil {
		return
	}
	h = &StorageHandler{Handler: b}
	return
}
