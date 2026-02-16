package hyperv

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

type Config struct {
	ProviderType api.ProviderType
	Root         string
}

func (c Config) ProviderRoot() string {
	return c.Root + "/:" + base.ProviderParam
}

type Resolver struct {
	*api.Provider
}

// Path builds the URL path for a resource.
func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	switch resource.(type) {
	case *VM:
		path = base.Link(VMRoot, base.Params{
			base.ProviderParam: string(r.Provider.UID),
			VMParam:            id,
		})
	case *Network:
		path = base.Link(NetworkRoot, base.Params{
			base.ProviderParam: string(r.Provider.UID),
			NetworkParam:       id,
		})
	case *Storage:
		path = base.Link(StorageRoot, base.Params{
			base.ProviderParam: string(r.Provider.UID),
			StorageParam:       id,
		})
	case *Disk:
		path = base.Link(DiskRoot, base.Params{
			base.ProviderParam: string(r.Provider.UID),
			DiskParam:          id,
		})
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: resource,
			},
		)
	}
	return
}

// Finder for HyperV providers.
type Finder struct {
	base.Client
	Config Config
}

// NewFinder creates a new HyperV finder.
func NewFinder() *Finder {
	return &Finder{
		Config: Config{
			ProviderType: api.HyperV,
			Root:         Root,
		},
	}
}

// With sets the REST client.
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
				return
			}
			if len(list) == 0 {
				err = base.NotFoundError{Ref: ref}
				return
			}
			if len(list) > 1 {
				err = base.RefNotUniqueError{Ref: ref}
				return
			}
			*res = list[0]
		} else {
			err = base.NotFoundError{Ref: ref}
		}
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
				return
			}
			if len(list) == 0 {
				err = base.NotFoundError{Ref: ref}
				return
			}
			if len(list) > 1 {
				err = base.RefNotUniqueError{Ref: ref}
				return
			}
			*res = list[0]
		} else {
			err = base.NotFoundError{Ref: ref}
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
				return
			}
			if len(list) == 0 {
				err = base.NotFoundError{Ref: ref}
				return
			}
			if len(list) > 1 {
				err = base.RefNotUniqueError{Ref: ref}
				return
			}
			*res = list[0]
		} else {
			err = base.NotFoundError{Ref: ref}
		}
	case *Storage:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Storage{}
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
				return
			}
			if len(list) == 0 {
				err = base.NotFoundError{Ref: ref}
				return
			}
			if len(list) > 1 {
				err = base.RefNotUniqueError{Ref: ref}
				return
			}
			*res = list[0]
		} else {
			err = base.NotFoundError{Ref: ref}
		}
	default:
		err = base.ResourceNotResolvedError{Object: resource}
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

// Find a Workload by ref.
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

// Find a Network by ref.
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

// Find a Storage by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	storage := &Storage{}
	err = r.ByRef(storage, *ref)
	if err == nil {
		ref.ID = storage.ID
		ref.Name = storage.Name
		object = storage
	}

	return
}

// Find a Host by ref.
// Returns the matching resource and:
//
//	ProviderNotSupportedErr
//	ProviderNotReadyErr
//	NotFoundErr
//	RefNotUniqueErr
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	err = base.ResourceNotResolvedError{Object: ref}
	return
}
