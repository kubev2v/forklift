package openstack

import (
	"strings"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

// Errors.
type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

// API path resolver.
type Resolver struct {
	*api.Provider
}

// Build the URL path.
func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	provider := r.Provider
	switch resource.(type) {
	case *Provider:
		r := Provider{}
		r.UID = id
		r.Link()
		path = r.SelfLink
	case *VM:
		r := VM{}
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Image:
		r := Image{}
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: resource,
			})
	}

	path = strings.TrimRight(path, "/")

	return
}

// Find a resource by ref.
// Returns:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	return nil
}

// Find a VM by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) VM(ref *base.Ref) (object interface{}, err error) {
	vm := &VM{}
	err = r.ByRef(vm, *ref)
	if err == nil {
		ref.ID = vm.ID
		ref.Name = vm.Name
		object = vm
	}

	return
}

// Resource finder.
type Finder struct {
	base.Client
}

// With client.
func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

// Find workload by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Workload(ref *base.Ref) (object interface{}, err error) {
	workload := &Workload{}
	err = r.ByRef(workload, *ref)
	if err == nil {
		ref.ID = workload.ID
		ref.Name = workload.Name
		object = workload
	}

	return
}

// REST Resource.
type Network struct {
	Resource
}

func (r *Finder) Network(ref *base.Ref) (object interface{}, err error) {
	network := &Network{}
	err = r.ByRef(network, *ref)
	if err == nil {
		ref.ID = network.ID
		ref.Name = network.Name
		object = network
	}

	return
}

// REST Resource.
type Storage struct {
	Resource
}

// Find storage by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	s := &Storage{}
	err = r.ByRef(s, *ref)
	if err == nil {
		ref.ID = s.ID
		ref.Name = s.Name
		object = s
	}

	return
}
