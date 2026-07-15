package web

import (
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

// Resolver builds URL paths for Azure resources.
// Uses the short resource name (extracted from the full Azure resource ID) as
// the path parameter, since full IDs contain slashes that break URL routing.
type Resolver struct {
	*api.Provider
}

func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	var kind string

	switch resource.(type) {
	case *VM, *[]VM:
		kind = "VM"
	case *Disk, *[]Disk:
		kind = "Disk"
	case *Network, *[]Network:
		kind = "Network"
	case *Storage, *[]Storage:
		kind = "Storage"
	case *Provider, *[]Provider:
		kind = "Provider"
	case *Workload, *[]Workload:
		kind = "VM"
	default:
		err = liberr.Wrap(
			base.ResourceNotResolvedError{
				Object: resource,
			})
		return
	}

	return r.PathForKind(kind, id)
}

func (r *Resolver) PathForKind(kind string, id string) (path string, err error) {
	provider := r.Provider
	providerUID := string(provider.UID)

	switch kind {
	case "VM":
		path = base.Link(VMRoot, base.Params{
			base.ProviderParam: providerUID,
			VMParam:            id,
		})
	case "Disk":
		path = base.Link(ProviderRoot+"/disks/:id", base.Params{
			base.ProviderParam: providerUID,
			"id":               id,
		})
	case "Network":
		path = base.Link(ProviderRoot+"/networks/:id", base.Params{
			base.ProviderParam: providerUID,
			"id":               id,
		})
	case "Storage":
		path = base.Link(ProviderRoot+"/storages/:id", base.Params{
			base.ProviderParam: providerUID,
			"id":               id,
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

var _ base.Resolver = &Resolver{}

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
		err = r.findByRef(res, ref)
	case *Disk:
		err = r.findByRef(res, ref)
	case *Network:
		err = r.findByRef(res, ref)
	case *Storage:
		err = r.findByRef(res, ref)
	case *Workload:
		err = r.findByRef(res, ref)
	default:
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
	}
	return
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

	switch resource.(type) {
	case *VM, *Workload:
		vmList := []VM{}
		err = r.List(
			&vmList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: ref.Name},
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
		if vm, ok := resource.(*VM); ok {
			*vm = vmList[0]
		} else if wl, ok := resource.(*Workload); ok {
			*wl = Workload{Resource: vmList[0].Resource}
		}
	case *Disk:
		diskList := []Disk{}
		err = r.List(
			&diskList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: ref.Name},
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
		*resource.(*Disk) = diskList[0]
	case *Network:
		networkList := []Network{}
		err = r.List(
			&networkList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: ref.Name},
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
		*resource.(*Network) = networkList[0]
	case *Storage:
		storageList := []Storage{}
		err = r.List(
			&storageList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: ref.Name},
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
		*resource.(*Storage) = storageList[0]
	}
	return
}

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

func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	storage := &Storage{}
	err = r.findByRef(storage, *ref)
	if err == nil {
		ref.ID = storage.ID
		ref.Name = storage.Name
		object = storage
	}
	return
}

func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	err = liberr.New("Host resources are not supported for Azure provider")
	return
}

var _ base.Finder = &Finder{}
