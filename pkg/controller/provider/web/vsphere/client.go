package vsphere

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	ocpmodel "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	pathlib "path"
	"strings"
)

//
// Errors.
type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

//
// API path resolver.
type Resolver struct {
	*api.Provider
}

//
// Build the URL path.
func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	switch resource.(type) {
	case *Provider:
		ns, name := pathlib.Split(id)
		ns = strings.TrimSuffix(ns, "/")
		if id == "/" { // list
			ns = r.Provider.Namespace
		}
		h := ProviderHandler{}
		path = h.Link(
			&ocpmodel.Provider{
				Base: ocpmodel.Base{
					Namespace: ns,
					Name:      name,
				},
			})
	case *Datacenter:
		h := DatacenterHandler{}
		path = h.Link(
			r.Provider,
			&model.Datacenter{
				Base: model.Base{ID: id},
			})
	case *Cluster:
		h := ClusterHandler{}
		path = h.Link(
			r.Provider,
			&model.Cluster{
				Base: model.Base{ID: id},
			})
	case *Host:
		h := HostHandler{}
		path = h.Link(
			r.Provider,
			&model.Host{
				Base: model.Base{ID: id},
			})
	case *Network:
		h := NetworkHandler{}
		path = h.Link(
			r.Provider,
			&model.Network{
				Base: model.Base{ID: id},
			})
	case *Datastore:
		h := DatastoreHandler{}
		path = h.Link(
			r.Provider,
			&model.Datastore{
				Base: model.Base{ID: id},
			})
	case *VM:
		h := VMHandler{}
		path = h.Link(
			r.Provider,
			&model.VM{
				Base: model.Base{ID: id},
			})
	case *Workload:
		h := WorkloadHandler{}
		path = h.Link(
			r.Provider,
			&model.VM{
				Base: model.Base{ID: id},
			})
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: resource,
			})
	}

	return
}

//
// Resource finder.
type Finder struct {
	base.Client
}

//
// With client.
func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

//
// Find a resource by ref.
// Returns:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	switch resource.(type) {
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
					Value: "1",
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
			*resource.(*Network) = list[0]
		}
	case *Datastore:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Datastore{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "1",
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
			*resource.(*Datastore) = list[0]
		}
	case *Host:
		id := ref.ID
		if id != "" {
			err = r.Get(resource, id)
			return
		}
		name := ref.Name
		if name != "" {
			list := []Host{}
			err = r.List(
				&list,
				base.Param{
					Key:   DetailParam,
					Value: "1",
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
			*resource.(*Host) = list[0]
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
					Value: "1",
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
			*resource.(*VM) = list[0]
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
					Value: "1",
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
			*resource.(*Workload) = list[0]
		}
	default:
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
	}

	return
}

//
// Find a VM by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
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

//
// Find workload by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
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

//
// Find a Network by ref.
//Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
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

//
// Find storage by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	ds := &Datastore{}
	err = r.ByRef(ds, *ref)
	if err == nil {
		ref.ID = ds.ID
		ref.Name = ds.Name
		object = ds
	}

	return
}

//
// Find host by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	host := &Host{}
	err = r.ByRef(host, *ref)
	if err == nil {
		ref.ID = host.ID
		ref.Name = host.Name
		object = host
	}

	return
}
