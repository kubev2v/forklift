package web

import (
	"fmt"
	"net/http"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Resource kind cannot be resolved.
type ProviderNotSupportedError struct {
	*api.Provider
}

func (r ProviderNotSupportedError) Error() string {
	return fmt.Sprintf("Provider (type) not supported: %#v", r.Provider)
}

// Resource kind cannot be resolved.
type ProviderNotReadyError struct {
	*api.Provider
}

func (r ProviderNotReadyError) Error() string {
	return fmt.Sprintf("Provider not ready: %#v", r.Provider)
}

type ConflictError struct {
	Provider *api.Provider
	Err      error
}

func (r ConflictError) Error() string {
	return fmt.Sprintf("Conflict Error from provider '%#v': %s", r.Provider, r.Err.Error())
}

type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

// Interfaces.
type EventHandler = base.EventHandler
type Client = base.Client
type Finder = base.Finder
type Param = base.Param
type Watch = base.Watch

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
	case api.OVirt:
		client = &ProviderClient{
			provider: provider,
			finder:   &ovirt.Finder{},
			restClient: base.RestClient{
				Resolver: &ovirt.Resolver{Provider: provider},
			},
		}
	case api.OpenStack:
		client = &ProviderClient{
			provider: provider,
			finder:   &openstack.Finder{},
			restClient: base.RestClient{
				Resolver: &openstack.Resolver{Provider: provider},
			},
		}
	case api.Ova:
		client = &ProviderClient{
			provider: provider,
			finder:   &ova.Finder{},
			restClient: base.RestClient{
				Resolver: &ova.Resolver{Provider: provider},
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

// Provider API client.
type ProviderClient struct {
	restClient base.RestClient
	// The provider.
	provider *api.Provider
	// Finder.
	finder Finder
}

// Finder.
func (r *ProviderClient) Finder() Finder {
	return r.finder.With(r)
}

// Get a resource.
// Returns:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
func (r *ProviderClient) Get(resource interface{}, id string) (err error) {
	status, err := r.restClient.Get(resource, id)
	if err == nil {
		err = r.asError(status, id)
	}

	return
}

// List a resource collection.
// Returns:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
func (r *ProviderClient) List(resource interface{}, param ...Param) (err error) {
	status, err := r.restClient.List(resource, param...)
	if err == nil {
		err = r.asError(status, "")
	}

	return
}

// Watch a resource.
// Returns:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
func (r *ProviderClient) Watch(resource interface{}, h EventHandler) (w *Watch, err error) {
	status, w, err := r.restClient.Watch(resource, h)
	if err == nil {
		err = r.asError(status, "")
	}

	return
}

// Find an object by ref.
// Returns:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *ProviderClient) Find(resource interface{}, ref base.Ref) (err error) {
	err = r.Finder().ByRef(resource, ref)
	return
}

// Find a VM by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *ProviderClient) VM(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().VM(ref)
}

// Find a workload by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *ProviderClient) Workload(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Workload(ref)
}

// Find a network by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *ProviderClient) Network(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Network(ref)
}

// Find a storage object by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *ProviderClient) Storage(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Storage(ref)
}

// Find a Host by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *ProviderClient) Host(ref *base.Ref) (object interface{}, err error) {
	return r.Finder().Host(ref)
}

// Evaluate the status.
// Returns:
//
//	ProviderNotReady
//	NotFound
func (r *ProviderClient) asError(status int, id string) (err error) {
	switch status {
	case http.StatusOK:
	case http.StatusPartialContent:
		err = liberr.Wrap(
			ProviderNotReadyError{
				r.provider,
			})
	case http.StatusNotFound:
		if r.HasReason(base.UnknownProvider) {
			err = liberr.Wrap(
				ProviderNotReadyError{
					r.provider,
				})
		} else {
			err = liberr.Wrap(
				NotFoundError{
					Ref: base.Ref{ID: id},
				})
		}
	default:
		err = liberr.New(http.StatusText(status))
	}

	return
}

// Match X-Reason reply header.
func (r *ProviderClient) HasReason(reason string) bool {
	reason = strings.ToLower(reason)
	if reasons, found := r.restClient.Reply.Header[base.ReasonHeader]; found {
		for i := range reasons {
			if strings.ToLower(reasons[i]) == reason {
				return true
			}
		}
	}

	return false
}
