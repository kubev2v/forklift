package web

import (
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"net/http"
)

//
// Resource kind cannot be resolved.
type ProviderNotSupportedError struct {
	*api.Provider
}

func (r ProviderNotSupportedError) Error() string {
	return fmt.Sprintf("Provider (type) not supported: %#v", r.Provider)
}

//
// Resource kind cannot be resolved.
type ProviderNotReadyError struct {
	*api.Provider
}

func (r ProviderNotReadyError) Error() string {
	return fmt.Sprintf("Provider not ready: %#v", r.Provider)
}

type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

//
// Interfaces.
type EventHandler = base.EventHandler
type Client = base.Client
type Finder = base.Finder
type Param = base.Param
type Watch = base.Watch

//
// Build an appropriate client.
func NewClient(provider *api.Provider) (client Client, err error) {
	switch provider.Type() {
	case api.OpenShift:
		client = &ProviderClient{
			provider: provider,
			finder:   &ocp.Finder{},
			restClient: base.RestClient{
				Resolver: &ocp.Resolver{Provider: provider},
			},
		}
	case api.VSphere:
		client = &ProviderClient{
			provider: provider,
			finder:   &vsphere.Finder{},
			restClient: base.RestClient{
				Resolver: &vsphere.Resolver{Provider: provider},
			},
		}
	default:
		err = liberr.Wrap(
			ProviderNotSupportedError{
				Provider: provider,
			})
	}

	return
}

//
// Provider API client.
type ProviderClient struct {
	restClient base.RestClient
	// The provider.
	provider *api.Provider
	// Finder.
	finder Finder
	// Ready.
	found bool
}

//
// Finder.
func (r *ProviderClient) Finder() Finder {
	return r.finder.With(r)
}

//
// Get a resource.
// Returns:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
func (r *ProviderClient) Get(resource interface{}, id string) (err error) {
	err = r.find()
	if err != nil {
		return
	}
	if !r.found {
		err = liberr.Wrap(ProviderNotReadyError{r.provider})
		return
	}
	status, err := r.restClient.Get(resource, id)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		err = liberr.Wrap(NotFoundError{Ref: base.Ref{ID: id}})
	default:
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// List a resource collection.
// Returns:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
func (r *ProviderClient) List(resource interface{}, param ...Param) (err error) {
	err = r.find()
	if err != nil {
		return
	}
	if !r.found {
		err = liberr.Wrap(ProviderNotReadyError{r.provider})
		return
	}
	status, err := r.restClient.List(resource, param...)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		err = liberr.Wrap(NotFoundError{})
	default:
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Watch a resource.
// Returns:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
func (r *ProviderClient) Watch(resource interface{}, h EventHandler) (w *Watch, err error) {
	err = r.find()
	if err != nil {
		return
	}
	if !r.found {
		err = liberr.Wrap(ProviderNotReadyError{r.provider})
		return
	}
	status, w, err := r.restClient.Watch(resource, h)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		err = liberr.Wrap(NotFoundError{})
	default:
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Find an object by ref.
// Returns:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *ProviderClient) Find(resource interface{}, ref base.Ref) (err error) {
	err = r.Finder().ByRef(resource, ref)
	return
}

//
// Find a VM by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *ProviderClient) VM(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().VM(ref)
}

//
// Find a workload by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *ProviderClient) Workload(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Workload(ref)
}

//
// Find a network by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *ProviderClient) Network(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Network(ref)
}

//
// Find a storage object by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *ProviderClient) Storage(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Storage(ref)
}

//
// Find a Host by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *ProviderClient) Host(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Host(ref)
}

//
// Find the provider.
func (r *ProviderClient) find() (err error) {
	if r.found {
		return
	}
	status := 0
	id := string(r.provider.UID)
	switch r.provider.Type() {
	case api.OpenShift:
		status, err = r.restClient.Get(&ocp.Provider{}, id)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.found = status == http.StatusOK
	case api.VSphere:
		status, err = r.restClient.Get(&vsphere.Provider{}, id)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.found = status == http.StatusOK
	default:
		err = liberr.Wrap(ProviderNotReadyError{r.provider})
	}

	return
}
