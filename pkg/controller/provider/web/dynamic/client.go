package dynamic

import (
	"errors"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

var (
	// ErrHostNotSupported is returned when host operations are not supported
	ErrHostNotSupported = errors.New("host operations not supported for dynamic providers")
)

// Errors.
type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

type Finder struct {
	base.Client
}

// With client
func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

// ByRef - Find a resource by ref
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	// For dynamic providers, query the cached inventory
	id := ref.ID
	if id != "" {
		// Fetch by ID
		err = r.Get(resource, id)
		return
	}

	// Fetch by name
	name := ref.Name
	if name == "" {
		err = liberr.Wrap(RefNotUniqueError{Ref: ref})
		return
	}

	// Query the resource collection by name
	// For dynamic providers, we use the model types from model/dynamic package
	switch res := resource.(type) {
	case *model.VM:
		if id != "" {
			err = r.Get(res, id)
		} else {
			list := []model.VM{}
			err = r.List(
				&list,
				base.Param{
					Key:   base.DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   base.NameParam,
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
	case *model.Network:
		if id != "" {
			err = r.Get(res, id)
		} else {
			list := []model.Network{}
			err = r.List(
				&list,
				base.Param{
					Key:   base.DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   base.NameParam,
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
	case *model.Storage:
		if id != "" {
			err = r.Get(res, id)
		} else {
			list := []model.Storage{}
			err = r.List(
				&list,
				base.Param{
					Key:   base.DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   base.NameParam,
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
	case *model.Disk:
		if id != "" {
			err = r.Get(res, id)
		} else {
			list := []model.Disk{}
			err = r.List(
				&list,
				base.Param{
					Key:   base.DetailParam,
					Value: "all",
				},
				base.Param{
					Key:   base.NameParam,
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
			base.ResourceNotResolvedError{
				Object: resource,
			})
	}

	return
}

func (r *Finder) VM(ref *base.Ref) (interface{}, error) {
	return &model.VM{}, nil
}

func (r *Finder) Workload(ref *base.Ref) (interface{}, error) {
	return &model.Workload{}, nil
}

func (r *Finder) Network(ref *base.Ref) (interface{}, error) {
	return &model.Network{}, nil
}

func (r *Finder) Storage(ref *base.Ref) (interface{}, error) {
	return &model.Storage{}, nil
}

func (r *Finder) Host(ref *base.Ref) (interface{}, error) {
	return nil, ErrHostNotSupported
}

type Resolver struct {
	Provider *api.Provider
}

func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	// Build path for dynamic providers
	// Format: /providers/:type/:provider/:collection/:id
	provider := r.Provider
	providerType := string(provider.Type())
	providerUID := string(provider.UID)

	var collection string
	switch resource.(type) {
	case *model.VM: // Note: model.Workload is an alias for model.VM, so both are covered here
		collection = "vms"
	case *model.Network:
		collection = "networks"
	case *model.Storage:
		collection = "storage"
	case *model.Disk:
		collection = "disks"
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: resource,
			})
		return
	}

	// Build path: /providers/:type/:provider/:collection/:id
	// For List operations, id will be "/", so we need to handle it specially
	if id == "" || id == "/" {
		path = fmt.Sprintf("/providers/%s/%s/%s", providerType, providerUID, collection)
	} else {
		path = fmt.Sprintf("/providers/%s/%s/%s/%s", providerType, providerUID, collection, id)
	}
	return
}
