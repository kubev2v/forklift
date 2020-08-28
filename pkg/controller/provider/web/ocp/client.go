package ocp

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
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
	// thin client
	client base.Client
}

//
// Get resources.
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
func (c *Client) Path(object interface{}, id string) (path string, err error) {
	ns, name := pathlib.Split(id)
	ns = strings.TrimSuffix(ns, "/")
	switch object.(type) {
	case *Provider:
		if id == "" { // list
			ns = c.Provider.Namespace
		}
		h := ProviderHandler{}
		path = h.Link(&model.Provider{
			Base: model.Base{
				Namespace: ns,
				Name:      name,
			},
		})
	default:
		err = liberr.New("unknown")
	}

	return
}
