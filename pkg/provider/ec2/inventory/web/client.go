package web

import (
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Errors.
type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

// API path resolver.
type Resolver struct {
	*api.Provider
}

// Build the URL path using kind instead of typed objects.
func (r *Resolver) PathForKind(kind string, id string) (path string, err error) {
	provider := r.Provider
	providerUID := string(provider.UID)

	switch kind {
	case "Instance":
		path = base.Link(VMRoot, base.Params{
			base.ProviderParam: providerUID,
			VMParam:            id,
		})
	case "Volume":
		path = base.Link(ProviderRoot+"/volumes/:id", base.Params{
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

// Build the URL path (required by base.Resolver interface).
// EC2 provider supports both typed resources (for internal use) and unstructured resources.
func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	var kind string

	// Try to extract kind from unstructured object first
	if res, ok := resource.(*unstructured.Unstructured); ok {
		var found bool
		kind, found, _ = unstructured.NestedString(res.Object, "kind")
		if !found || kind == "" {
			err = liberr.Wrap(
				base.ResourceNotResolvedError{
					Object: resource,
				})
			return
		}
	} else {
		// Handle typed resources by checking their type
		switch resource.(type) {
		case *VM, *[]VM:
			kind = "Instance"
		case *Volume, *[]Volume:
			kind = "Volume"
		case *Network, *[]Network:
			kind = "Network"
		case *Storage, *[]Storage:
			kind = "Storage"
		case *Provider:
			kind = "Provider"
		case *Workload, *[]Workload:
			kind = "Instance" // Workload uses the same endpoint as VM
		default:
			err = liberr.Wrap(
				base.ResourceNotResolvedError{
					Object: resource,
				})
			return
		}
	}

	return r.PathForKind(kind, id)
}

// Compile-time interface check - zero-cost type safety
// Verify that *Resolver implements base.Resolver (1 method: Path)
var _ base.Resolver = &Resolver{}

// Resource finder.
type Finder struct {
	base.Client
}

// With client.
func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

// ByRef finds resource by ref. EC2 only supports unstructured resources.
// Returns: ProviderNotSupportedErr, ProviderNotReadyErr, NotFoundErr, RefNotUniqueErr, ResourceNotResolvedError.
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	res, ok := resource.(*unstructured.Unstructured)
	if !ok {
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
		return
	}

	// Check if the unstructured object already has a kind field
	kind, found, _ := unstructured.NestedString(res.Object, "kind")
	if !found || kind == "" {
		// If no kind field, we cannot determine the resource type
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
		return
	}

	// Use the kind field to determine which resource type to fetch
	switch kind {
	case "Instance":
		vm := &VM{}
		err = r.findTypedResource(vm, ref)
		if err == nil {
			res.SetUnstructuredContent(map[string]interface{}{
				"id":       vm.ID,
				"name":     vm.Name,
				"kind":     "Instance",
				"revision": vm.Revision,
				"path":     vm.Path,
				"object":   vm.Object,
			})
		}
	case "Volume":
		volume := &Volume{}
		err = r.findTypedResource(volume, ref)
		if err == nil {
			res.SetUnstructuredContent(map[string]interface{}{
				"id":       volume.ID,
				"name":     volume.Name,
				"kind":     "Volume",
				"revision": volume.Revision,
				"path":     volume.Path,
				"object":   volume.Object,
			})
		}
	case "Network":
		network := &Network{}
		err = r.findTypedResource(network, ref)
		if err == nil {
			res.SetUnstructuredContent(map[string]interface{}{
				"id":       network.ID,
				"name":     network.Name,
				"kind":     "Network",
				"revision": network.Revision,
				"path":     network.Path,
				"object":   network.Object,
			})
		}
	case "Storage":
		storage := &Storage{}
		err = r.findTypedResource(storage, ref)
		if err == nil {
			res.SetUnstructuredContent(map[string]interface{}{
				"id":       storage.ID,
				"name":     storage.Name,
				"kind":     "Storage",
				"revision": storage.Revision,
				"path":     storage.Path,
				"object":   storage.Object,
			})
		}
	default:
		// Unknown kind
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: resource,
			})
	}

	return
}

// Internal helper to find a typed resource by ref
func (r *Finder) findTypedResource(resource interface{}, ref base.Ref) (err error) {
	// Try by ID first
	if ref.ID != "" {
		err = r.Get(resource, ref.ID)
		return
	}

	// If no ID, try by name
	if ref.Name == "" {
		err = liberr.Wrap(NotFoundError{Ref: ref})
		return
	}

	// Use type switch to create the appropriate list type
	switch res := resource.(type) {
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
		if vm, ok := res.(*VM); ok {
			*vm = vmList[0]
		} else if wl, ok := res.(*Workload); ok {
			*wl = Workload{Resource: vmList[0].Resource}
		}
	case *Volume:
		volumeList := []Volume{}
		err = r.List(
			&volumeList,
			base.Param{Key: base.DetailParam, Value: "all"},
			base.Param{Key: base.NameParam, Value: ref.Name},
		)
		if err != nil {
			return
		}
		if len(volumeList) == 0 {
			err = liberr.Wrap(NotFoundError{Ref: ref})
			return
		}
		if len(volumeList) > 1 {
			err = liberr.Wrap(RefNotUniqueError{Ref: ref})
			return
		}
		*res = volumeList[0]
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
		*res = networkList[0]
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
		*res = storageList[0]
	}
	return
}

// Find a VM by ref.
// VM finds VM by ref. Returns: ProviderNotSupportedErr, ProviderNotReadyErr, NotFoundErr, RefNotUniqueErr.
func (r *Finder) VM(ref *base.Ref) (object interface{}, err error) {
	vm := &VM{}
	err = r.findTypedResource(vm, *ref)
	if err == nil {
		ref.ID = vm.ID
		ref.Name = vm.Name
		object = vm
	}

	return
}

// Workload finds workload by ref. Returns: ProviderNotSupportedErr, ProviderNotReadyErr, NotFoundErr, RefNotUniqueErr.
func (r *Finder) Workload(ref *base.Ref) (object interface{}, err error) {
	workload := &Workload{}
	err = r.findTypedResource(workload, *ref)
	if err == nil {
		ref.ID = workload.ID
		ref.Name = workload.Name
		object = workload
	}

	return
}

// Network finds network by ref. Returns: ProviderNotSupportedErr, ProviderNotReadyErr, NotFoundErr, RefNotUniqueErr.
func (r *Finder) Network(ref *base.Ref) (object interface{}, err error) {
	network := &Network{}
	err = r.findTypedResource(network, *ref)
	if err == nil {
		ref.ID = network.ID
		ref.Name = network.Name
		object = network
	}

	return
}

// Storage finds storage by ref. Returns: ProviderNotSupportedErr, ProviderNotReadyErr, NotFoundErr, RefNotUniqueErr.
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	storage := &Storage{}
	err = r.findTypedResource(storage, *ref)
	if err == nil {
		ref.ID = storage.ID
		ref.Name = storage.Name
		object = storage
	}

	return
}
