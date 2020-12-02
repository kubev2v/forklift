package web

import (
	"errors"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"net/http"
	"path"
)

var (
	ProviderNotSupportedErr = errors.New("provider (type) not supported")
	ProviderNotReadyErr     = errors.New("provider API not ready")
)

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
}

//
// Build an appropriate client.
func NewClient(provider *api.Provider) (client Client, err error) {
	switch provider.Type() {
	case api.OpenShift:
		client = &ProviderClient{
			provider: provider,
			Client: base.Client{
				Resolver: &ocp.Resolver{Provider: provider},
			},
		}
	case api.VSphere:
		client = &ProviderClient{
			provider: provider,
			Client: base.Client{
				Resolver: &vsphere.Resolver{Provider: provider},
			},
		}
	default:
		err = liberr.Wrap(ProviderNotSupportedErr)
	}

	return
}

//
// Provider API client.
type ProviderClient struct {
	base.Client
	// The provider.
	provider *api.Provider
	// Ready.
	found bool
}

//
// Get a resource.
// Raises ProviderNotReadyErr on 404 and 206
// when the provider is not yet in the inventory.
func (r *ProviderClient) Get(resource interface{}, id string) (status int, err error) {
	err = r.Find()
	if err != nil {
		return
	}
	if !r.found {
		err = liberr.Wrap(ProviderNotReadyErr)
		return
	}
	status, err = r.Client.Get(resource, id)

	return
}

//
// List a resource collection.
// Raises ProviderNotReadyErr on 404 and 206
// when the provider is not yet in the inventory.
func (r *ProviderClient) List(resource interface{}) (status int, err error) {
	err = r.Find()
	if err != nil {
		return
	}
	if !r.found {
		err = liberr.Wrap(ProviderNotReadyErr)
		return
	}
	status, err = r.Client.List(resource)

	return
}

//
// Find the provider.
func (r *ProviderClient) Find() (err error) {
	if r.found {
		return
	}
	id := path.Join(
		r.provider.Namespace,
		r.provider.Name)
	status := 0
	switch r.provider.Type() {
	case api.OpenShift:
		status, err = r.Client.Get(&ocp.Provider{}, id)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.found = status == http.StatusOK
	case api.VSphere:
		status, err = r.Client.Get(&vsphere.Provider{}, id)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.found = status == http.StatusOK
	default:
		err = liberr.Wrap(ProviderNotSupportedErr)
	}

	return
}
