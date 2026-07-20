package nutanix

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
	switch resource.(type) {
	case *Provider:
		res := Provider{}
		res.UID = id
		res.Link()
		path = res.SelfLink
	case *Cluster:
		res := Cluster{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
	case *Host:
		res := Host{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
	case *Network:
		res := Network{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
	case *StorageContainer:
		res := StorageContainer{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
	case *Image:
		res := Image{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
	case *VM:
		res := VM{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
	case *Workload:
		res := Workload{}
		res.ID = id
		res.Link(provider)
		path = res.SelfLink
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

func (r *Finder) findByRef(resource interface{}, ref base.Ref) (err error) {
	if ref.ID != "" {
		err = r.Get(resource, ref.ID)
		return
	}
	if ref.Name == "" {
		err = liberr.Wrap(NotFoundError{Ref: ref})
		return
	}

	switch res := resource.(type) {
	case *VM:
		list := []VM{}
		err = r.listByName(&list, ref.Name)
		if err != nil {
			return
		}
		if len(list) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(list) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = list[0]
	case *Workload:
		list := []Workload{}
		err = r.listByName(&list, ref.Name)
		if err != nil {
			return
		}
		if len(list) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(list) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = list[0]
	case *Network:
		list := []Network{}
		err = r.listByName(&list, ref.Name)
		if err != nil {
			return
		}
		if len(list) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(list) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = list[0]
	case *StorageContainer:
		list := []StorageContainer{}
		err = r.listByName(&list, ref.Name)
		if err != nil {
			return
		}
		if len(list) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(list) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = list[0]
	case *Host:
		list := []Host{}
		err = r.listByName(&list, ref.Name)
		if err != nil {
			return
		}
		if len(list) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(list) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = list[0]
	case *Cluster:
		list := []Cluster{}
		err = r.listByName(&list, ref.Name)
		if err != nil {
			return
		}
		if len(list) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(list) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = list[0]
	default:
		err = liberr.Wrap(ResourceNotResolvedError{Object: resource})
	}

	return
}

func (r *Finder) listByName(list interface{}, name string) error {
	return r.List(
		list,
		base.Param{
			Key:   DetailParam,
			Value: "all",
		},
		base.Param{
			Key:   NameParam,
			Value: name,
		})
}

// Find a resource by ref.
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	return r.findByRef(resource, ref)
}

// VM finds VM by ref.
func (r *Finder) VM(ref *base.Ref) (object interface{}, err error) {
	vm := &VM{}
	err = r.findByRef(vm, *ref)
	if err == nil {
		ref.ID = vm.ID
		ref.Name = vm.Name
		object = vm
	}

	return
}

// Workload finds a workload by ref.
func (r *Finder) Workload(ref *base.Ref) (object interface{}, err error) {
	workload := &Workload{}
	err = r.findByRef(workload, *ref)
	if err == nil {
		ref.ID = workload.ID
		ref.Name = workload.Name
		object = workload
	}

	return
}

// Network finds network by ref.
func (r *Finder) Network(ref *base.Ref) (object interface{}, err error) {
	network := &Network{}
	err = r.findByRef(network, *ref)
	if err == nil {
		ref.ID = network.ID
		ref.Name = network.Name
		object = network
	}

	return
}

// Storage finds storage container by ref.
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	storage := &StorageContainer{}
	err = r.findByRef(storage, *ref)
	if err == nil {
		ref.ID = storage.ID
		ref.Name = storage.Name
		object = storage
	}

	return
}

// Host finds host by ref.
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	host := &Host{}
	err = r.findByRef(host, *ref)
	if err == nil {
		ref.ID = host.ID
		ref.Name = host.Name
		object = host
	}

	return
}
