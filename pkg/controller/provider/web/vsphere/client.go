package vsphere

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	ocpmodel "github.com/konveyor/virt-controller/pkg/controller/provider/model/ocp"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
	pathlib "path"
	"reflect"
	"strings"
)

//
// Web client.
type Client struct {
	// Host <host>:<port>
	Host string
	// Provider
	Provider api.Provider
}

//
// Get a resource.
func (c *Client) Get(resource interface{}, id string) (int, error) {
	lv := reflect.ValueOf(resource)
	switch lv.Kind() {
	case reflect.Ptr:
	default:
		return -1, libmodel.MustBePtrErr
	}
	client := base.Client{
		Host: c.Host,
	}
	path, err := c.Path(resource, id)
	if err != nil {
		return -1, liberr.Wrap(err)
	}
	status, err := client.Get(path, resource)
	if err != nil {
		return -1, liberr.Wrap(err)
	}

	return status, nil
}

//
// List resources.
func (c *Client) List(list interface{}) (int, error) {
	var resource interface{}
	lt := reflect.TypeOf(list)
	lv := reflect.ValueOf(list)
	switch lv.Kind() {
	case reflect.Ptr:
		lt := lt.Elem()
		lv = lv.Elem()
		switch lv.Kind() {
		case reflect.Slice:
			resource = reflect.New(lt.Elem()).Interface()
		default:
			return -1, libmodel.MustBeSlicePtrErr
		}
	default:
		return -1, libmodel.MustBeSlicePtrErr
	}
	client := base.Client{
		Host: c.Host,
	}
	path, err := c.Path(resource, "")
	if err != nil {
		return -1, liberr.Wrap(err)
	}
	status, err := client.Get(path, list)
	if err != nil {
		return -1, liberr.Wrap(err)
	}

	return status, nil
}

// Get whether the provider has been reconciled.
func (c *Client) Ready() (found bool, err error) {
	p := &Provider{}
	id := pathlib.Join(
		c.Provider.Namespace,
		c.Provider.Name)
	status, err := c.Get(p, id)
	if err != nil {
		return
	}

	found = status != http.StatusNotFound

	return
}

//
// Build the URL path.
func (c *Client) Path(resource interface{}, id string) (path string, err error) {
	switch resource.(type) {
	case *Provider:
		ns, name := pathlib.Split(id)
		ns = strings.TrimSuffix(ns, "/")
		if id == "" { // list
			ns = c.Provider.Namespace
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
			&c.Provider,
			&model.Datacenter{
				Base: model.Base{ID: id},
			})
	case *Cluster:
		h := ClusterHandler{}
		path = h.Link(
			&c.Provider,
			&model.Cluster{
				Base: model.Base{ID: id},
			})
	case *Host:
		h := HostHandler{}
		path = h.Link(
			&c.Provider,
			&model.Host{
				Base: model.Base{ID: id},
			})
	case *Network:
		h := NetworkHandler{}
		path = h.Link(
			&c.Provider,
			&model.Network{
				Base: model.Base{ID: id},
			})
	case *Datastore:
		h := DatastoreHandler{}
		path = h.Link(
			&c.Provider,
			&model.Datastore{
				Base: model.Base{ID: id},
			})
	case *VM:
		h := VMHandler{}
		path = h.Link(
			&c.Provider,
			&model.VM{
				Base: model.Base{ID: id},
			})
	default:
		err = base.ResourceNotSupported
	}

	return
}
