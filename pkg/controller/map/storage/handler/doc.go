package handler

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/map/storage/handler/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/map/storage/handler/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/map/storage/handler/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/watch/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type WatchManager = handler.WatchManager

//
// Inventory event handler.
type Handler interface {
	// Ensure watch started.
	Watch(m *handler.WatchManager) error
}

//
// Handler factory.
func New(
	client client.Client,
	channel chan event.GenericEvent,
	provider *api.Provider) (h Handler, err error) {
	//
	switch provider.Type() {
	case api.OpenShift:
		h, err = ocp.New(
			client,
			channel,
			provider)
	case api.VSphere:
		h, err = vsphere.New(
			client,
			channel,
			provider)
	case api.OVirt:
		h, err = ovirt.New(
			client,
			channel,
			provider)
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
