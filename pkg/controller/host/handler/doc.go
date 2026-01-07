package handler

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/host/handler/ocp"
	"github.com/kubev2v/forklift/pkg/controller/host/handler/openstack"
	"github.com/kubev2v/forklift/pkg/controller/host/handler/ova"
	"github.com/kubev2v/forklift/pkg/controller/host/handler/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/host/handler/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	ec2handler "github.com/kubev2v/forklift/pkg/provider/ec2/controller/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type WatchManager = handler.WatchManager

// Inventory event handler.
type Handler interface {
	// Ensure watch started.
	Watch(m *handler.WatchManager) error
}

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
	case api.OpenStack:
		h, err = openstack.New(
			client,
			channel,
			provider)
	case api.Ova:
		h, err = ova.New(
			client,
			channel,
			provider)
	case api.EC2:
		// EC2 provider does not support host-level operations
		// Return a no-op handler that satisfies the interface
		h = &ec2handler.NoOpHostHandler{}
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
