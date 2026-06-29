package web

import (
	"net/url"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

// API path resolver.
type Resolver struct {
	*api.Provider
}

func (r *Resolver) PathForKind(kind string, id string) (path string, err error) {
	provider := r.Provider
	providerUID := string(provider.UID)
	encodedID := url.PathEscape(id)

	switch kind {
	case "azure-vm":
		path = base.Link(VMRoot, base.Params{
			base.ProviderParam: providerUID,
			VMParam:            encodedID,
		})
	case "azure-disk":
		path = base.Link(ProviderRoot+"/disks/:id", base.Params{
			base.ProviderParam: providerUID,
			"id":               encodedID,
		})
	case "azure-network":
		path = base.Link(ProviderRoot+"/networks/:id", base.Params{
			base.ProviderParam: providerUID,
			"id":               encodedID,
		})
	case "azure-disk-type":
		path = base.Link(ProviderRoot+"/storages/:id", base.Params{
			base.ProviderParam: providerUID,
			"id":               encodedID,
		})
	case "Provider":
		path = base.Link(ProviderRoot, base.Params{
			base.ProviderParam: id,
		})
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: kind,
			})
		return
	}

	path = strings.TrimRight(path, "/")
	return
}

func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	var kind string

	switch resource.(type) {
	case *VM, *[]VM:
		kind = "azure-vm"
	case *Disk, *[]Disk:
		kind = "azure-disk"
	case *Network, *[]Network:
		kind = "azure-network"
	case *Storage, *[]Storage:
		kind = "azure-disk-type"
	case *Provider, *[]Provider:
		kind = "Provider"
	case *Workload, *[]Workload:
		kind = "azure-vm"
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: resource,
			})
		return
	}

	return r.PathForKind(kind, id)
}

var _ base.Resolver = &Resolver{}

// Resource finder.
type Finder struct {
	base.Client
}

func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	switch res := resource.(type) {
	case *VM:
		err = r.findTypedResource(res, ref)
	case *Disk:
		err = r.findTypedResource(res, ref)
	case *Network:
		err = r.findTypedResource(res, ref)
	case *Storage:
		err = r.findTypedResource(res, ref)
	case *Workload:
		err = r.findTypedResource(res, ref)
	default:
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
	}

	return
}

func (r *Finder) findTypedResource(resource interface{}, ref base.Ref) (err error) {
	// Azure ARM resource IDs contain slashes (e.g. /subscriptions/.../virtualMachines/vm-name)
	// which are incompatible with HTTP path parameters. Always use name-based list lookup.
	name := ref.Name
	if name == "" && ref.ID != "" {
		// Extract VM name from ARM resource ID (last segment)
		parts := strings.Split(ref.ID, "/")
		name = parts[len(parts)-1]
	}
	if name == "" {
		err = liberr.Wrap(NotFoundError{Ref: ref})
		return
	}

	switch res := resource.(type) {
	case *VM, *Workload:
		vmList := []VM{}
		err = r.List(
			&vmList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: name},
		)
		if err != nil {
			return
		}
		if len(vmList) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(vmList) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		if vm, ok := res.(*VM); ok {
			*vm = vmList[0]
		} else if wl, ok := res.(*Workload); ok {
			*wl = Workload{Resource: vmList[0].Resource}
		}
	case *Disk:
		diskList := []Disk{}
		err = r.List(
			&diskList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: name},
		)
		if err != nil {
			return
		}
		if len(diskList) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(diskList) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = diskList[0]
	case *Network:
		networkList := []Network{}
		err = r.List(
			&networkList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: name},
		)
		if err != nil {
			return
		}
		if len(networkList) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(networkList) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = networkList[0]
	case *Storage:
		storageList := []Storage{}
		err = r.List(
			&storageList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: name},
		)
		if err != nil {
			return
		}
		if len(storageList) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(storageList) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = storageList[0]
	}
	return
}

func (r *Finder) VM(ref *base.Ref) (object interface{}, err error) {
	vm := &VM{}
	err = r.findTypedResource(vm, *ref)
	if err == nil {
		ref.ID = vm.Name
		ref.Name = vm.Name
		object = vm
	}

	return
}

func (r *Finder) Workload(ref *base.Ref) (object interface{}, err error) {
	workload := &Workload{}
	err = r.findTypedResource(workload, *ref)
	if err == nil {
		ref.ID = workload.Name
		ref.Name = workload.Name
		object = workload
	}

	return
}

func (r *Finder) Network(ref *base.Ref) (object interface{}, err error) {
	network := &Network{}
	err = r.findTypedResource(network, *ref)
	if err == nil {
		ref.ID = network.Name
		ref.Name = network.Name
		object = network
	}

	return
}

func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	storage := &Storage{}
	err = r.findTypedResource(storage, *ref)
	if err == nil {
		ref.ID = storage.Name
		ref.Name = storage.Name
		object = storage
	}

	return
}
