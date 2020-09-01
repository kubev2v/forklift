package web

import (
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
)

var (
	ProviderNotSupported = base.ProviderNotSupported
)

//
// Build an appropriate client.
func NewClient(provider api.Provider) (client Client, err error) {
	switch provider.Type() {
	case api.OpenShift:
		client = &ocp.Client{
			Provider: provider,
		}
	case api.VSphere:
		client = &vsphere.Client{
			Provider: provider,
		}
	default:
		err = ProviderNotSupported
	}

	return
}

//
// REST Client.
type Client interface {
	// Get a resource.
	// The `resource` must be a pointer to a resource object.
	// Returns: The HTTP code and error.
	Get(resource interface{}, id string) (int, error)
	// List a collection.
	// The `list` must be a pointer to a slice of resource object.
	// Returns: The HTTP code and error.
	List(list interface{}) (int, error)
	// Get whether the provider is in inventory.
	// Returns: True when found.
	Ready() (bool, error)
}
