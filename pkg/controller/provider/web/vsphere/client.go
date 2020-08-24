package web

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/ocp"
	"reflect"
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

//
// Build the URL path.
func (c *Client) Path(resource interface{}, id string) (path string, err error) {
	switch resource.(type) {
	case *ocp.Provider:
		client := ocp.Client{
			Provider: c.Provider,
		}
		return client.Path(resource, id)
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
		err = liberr.New("unknown")
	}

	return
}
