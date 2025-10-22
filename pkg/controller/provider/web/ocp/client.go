package ocp

import (
	"path"
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

// Resolve the URL path.
func (r *Resolver) Path(object interface{}, id string) (path string, err error) {
	provider := r.Provider
	switch r := object.(type) {
	case *Provider:
		r.UID = id
		r.Link()
		path = r.SelfLink
	case *Namespace:
		r.UID = id
		r.Link(provider)
		path = r.SelfLink
	case *StorageClass:
		r.UID = id
		r.Link(provider)
		path = r.SelfLink
	case *NetworkAttachmentDefinition:
		r.UID = id
		r.Link(provider)
		path = r.SelfLink
	case *InstanceType:
		r.UID = id
		r.Link(provider)
		path = r.SelfLink
	case *ClusterInstanceType:
		r.UID = id
		r.Link(provider)
		path = r.SelfLink
	case *VM:
		r.UID = id
		r.Link(provider)
		path = r.SelfLink
	default:
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: object,
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
	case *NetworkAttachmentDefinition:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			ns, name := path.Split(name)
			ns = strings.TrimRight(ns, "/")
			list := []NetworkAttachmentDefinition{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NsParam,
					Value: ns,
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
	case *StorageClass:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []StorageClass{}
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
	case *InstanceType:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []InstanceType{}
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
	case *ClusterInstanceType:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []ClusterInstanceType{}
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
	case *VM:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			var ns string

			// ref.Namespace might be missing when passed from NetworkMaps
			// or StorageMaps
			if ref.Namespace != "" {
				ns = ref.Namespace
			} else {
				ns, name = path.Split(name)
				ns = strings.TrimRight(ns, "/")
			}
			list := []VM{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   NsParam,
					Value: ns,
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
		ref.ID = vm.UID

		if ref.Namespace == "" {
			ref.Name = path.Join(vm.Namespace, vm.Name)
		}

		object = vm
	}

	return
}

// Find workload by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Workload(ref *base.Ref) (object interface{}, err error) {
	vm := &VM{}
	err = r.ByRef(vm, *ref)
	if err == nil {
		ref.ID = vm.UID
		ref.Name = path.Join(vm.Namespace, vm.Name)
		object = vm
	}

	return
}

// Find a Network by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Network(ref *base.Ref) (object interface{}, err error) {
	nad := &NetworkAttachmentDefinition{}
	err = r.ByRef(nad, *ref)
	if err == nil {
		ref.ID = nad.UID
		ref.Name = path.Join(nad.Namespace, nad.Name)
		object = nad
	}

	return
}

// Find storage by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	sc := &StorageClass{}
	err = r.ByRef(sc, *ref)
	if err == nil {
		ref.ID = sc.UID
		ref.Name = sc.Name
		object = sc
	}

	return
}

// Find host by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	err = liberr.Wrap(&NotFoundError{
		Ref: *ref,
	})
	return
}

// Find a InstanceType by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) InstanceType(ref *base.Ref) (object interface{}, err error) {
	it := &InstanceType{}
	err = r.ByRef(it, *ref)
	if err == nil {
		ref.ID = it.UID
		ref.Name = path.Join(it.Namespace, it.Name)
		object = it
	}

	return
}

// Find a ClusterInstanceType by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) ClusterInstanceType(ref *base.Ref) (object interface{}, err error) {
	it := &ClusterInstanceType{}
	err = r.ByRef(it, *ref)
	if err == nil {
		ref.ID = it.UID
		ref.Name = it.Name
		object = it
	}

	return
}
