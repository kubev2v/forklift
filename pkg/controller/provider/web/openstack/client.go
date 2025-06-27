package openstack

import (
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
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
	switch r := resource.(type) {
	case *Provider:
		r.UID = id
		r.Link()
		path = r.SelfLink
	case *Region:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Project:
		r.ID = id
		r.Link(provider)
	case *Image:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Flavor:
		r.ID = id
		r.Link(provider)
	case *VM:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Snapshot:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Volume:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *VolumeType:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Network:
		r.ID = id
		r.Link(provider)
		path = r.SelfLink
	case *Workload:
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

// Resource finder.
type Finder struct {
	base.Client
}

// With client.
func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

// Find a resource by ref.
// Returns:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	switch res := resource.(type) {
	case *VM:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []VM{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	case *Workload:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Workload{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	case *Network:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Network{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	case *VolumeType:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []VolumeType{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	case *Snapshot:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Snapshot{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	case *Volume:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Volume{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	case *Image:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Image{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NameParam,
					Value: name,
				})
			if err != nil {
				break
			}
			if len(list) == 0 {
				err = liberr.Wrap(NotFoundError{Ref: ref})
				break
			}
			if len(list) > 1 {
				err = liberr.Wrap(RefNotUniqueError{Ref: ref})
				break
			}
			*res = list[0]
		}
	default:
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
	}

	return
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

// Find Workload by ref.
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

// Find Network by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
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

// Find a storage by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	if ref.Name == api.GlanceSource {
		return
	}

	volumeType := &VolumeType{}
	err = r.ByRef(volumeType, *ref)
	if err == nil {
		ref.ID = volumeType.ID
		ref.Name = volumeType.Name
		object = volumeType
	}

	return
}
