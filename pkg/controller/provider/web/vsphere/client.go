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
		if id == "" { // list
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
	default:
		err = liberr.Wrap(base.ResourceNotResolvedErr)
	}

	return
}
