package base

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"io/ioutil"
	"net/http"
	liburl "net/url"
	"reflect"
)

//
// Application settings.
var Settings = &settings.Settings

//
// Lib.
type EventHandler = libweb.EventHandler
type LibClient = libweb.Client
type Watch = libweb.Watch

//
// Resource kind cannot be resolved.
type ResourceNotResolvedError struct {
	Object interface{}
}

func (r ResourceNotResolvedError) Error() string {
	return fmt.Sprintf("Resource %#v cannot be resolved.", r.Object)
}

//
// Reference matches multiple resources.
type RefNotUniqueError struct {
	Ref
}

func (r RefNotUniqueError) Error() string {
	return fmt.Sprintf("Reference %#v matched multiple resources.", r.Ref)
}

//
// Resource not found.
type NotFoundError struct {
	Ref
}

func (r NotFoundError) Error() string {
	return fmt.Sprintf("Resource %#v not found.", r.Ref)
}

//
// Reference.
type Ref = ref.Ref

//
// Resolves resources to API paths.
type Resolver interface {
	// Find the API path for the specified resource.
	Path(resource interface{}, id string) (string, error)
}

//
// Resource Finder.
type Finder interface {
	// Finder with client.
	With(client Client) Finder
	// Find a resource by ref.
	// Returns:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	ByRef(resource interface{}, ref Ref) error
	// Find a VM by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	VM(ref *Ref) (interface{}, error)
	// Find a workload by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Workload(ref *Ref) (interface{}, error)
	// Find a Network by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Network(ref *Ref) (interface{}, error)
	// Find storage by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Storage(ref *Ref) (interface{}, error)
	// Find host by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Host(ref *Ref) (interface{}, error)
}

//
// REST Client.
type Client interface {
	// Finder
	Finder() Finder
	// Get a resource.
	// The `resource` must be a pointer to a resource object.
	// Returns:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	Get(resource interface{}, id string) error
	// List a collection.
	// The `list` must be a pointer to a slice of resource object.
	// Returns:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	List(list interface{}, param ...Param) error
	// Watch a collection.
	// Returns:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	Watch(resource interface{}, h EventHandler) (*Watch, error)
	// Get a resource by ref.
	// Returns:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Find(resource interface{}, ref Ref) error
	// Find a VM by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	VM(ref *Ref) (interface{}, error)
	// Find a Workload by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Workload(ref *Ref) (interface{}, error)
	// Find a Network by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Network(ref *Ref) (interface{}, error)
	// Find storage by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Storage(ref *Ref) (interface{}, error)
	// Find host by ref.
	// Returns the matching resource and:
	//   ProviderNotSupportedErr
	//   ProviderNotReadyErr
	//   NotFoundErr
	//   RefNotUniqueErr
	Host(ref *Ref) (interface{}, error)
}

//
// Web parameter.
type Param struct {
	Key   string
	Value string
}

//
// REST API client.
type RestClient struct {
	LibClient
	Resolver
	// Bearer token.
	Token string
	// Host <host>:<port>
	Host string
	// Parameters
	Params Params
}

//
// Get a resource.
func (c *RestClient) Get(resource interface{}, id string) (status int, err error) {
	if c.Resolver == nil {
		err = liberr.Wrap(ResourceNotResolvedError{resource})
		return
	}
	lv := reflect.ValueOf(resource)
	switch lv.Kind() {
	case reflect.Ptr:
	default:
		return -1, libmodel.MustBePtrErr
	}
	path, err := c.Resolver.Path(resource, id)
	if err != nil {
		return
	}

	status, err = c.get(path, resource)

	return
}

//
// List resources in a collection.
func (c *RestClient) List(list interface{}, param ...Param) (status int, err error) {
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
	path, err := c.Resolver.Path(resource, "/")
	if err != nil {
		return
	}
	if len(param) > 0 {
		q := liburl.Values{}
		for _, p := range param {
			q.Add(p.Key, p.Value)
		}
		path += "?" + q.Encode()
	}

	status, err = c.get(path, list)

	return
}

//
// Watch a resource.
func (c *RestClient) Watch(resource interface{}, h EventHandler) (status int, w *Watch, err error) {
	if c.Resolver == nil {
		err = liberr.Wrap(ResourceNotResolvedError{resource})
		return
	}
	lv := reflect.ValueOf(resource)
	switch lv.Kind() {
	case reflect.Ptr:
	default:
		err = libmodel.MustBePtrErr
		return
	}
	path, err := c.Resolver.Path(resource, "/")
	if err != nil {
		return
	}
	err = c.buildTransport()
	if err != nil {
		return
	}
	c.buildHeader()
	url := c.url(path)
	status, w, err = c.LibClient.Watch(url, resource, h)

	return
}

//
// Build and set the transport as needed.
func (c *RestClient) buildTransport() (err error) {
	if c.Transport != nil {
		return
	}
	if !Settings.Inventory.TLS.Enabled {
		c.Transport = http.DefaultTransport
		return
	}
	pool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(Settings.Inventory.TLS.CA)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	pool.AppendCertsFromPEM(ca)
	c.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}

	return
}

//
// Build header.
func (c *RestClient) buildHeader() {
	if c.Token == "" {
		return
	}
	c.Header = http.Header{
		"Authorization": []string{
			fmt.Sprintf("Bearer %s", c.Token),
		},
	}
}

//
// Build the URL.
func (c *RestClient) url(path string) string {
	if c.Host == "" {
		c.Host = fmt.Sprintf(
			"%s:%d",
			Settings.Inventory.Host,
			Settings.Inventory.Port)
	}
	path = (&Handler{}).Link(path, c.Params)
	url, _ := liburl.Parse(path)
	if url.Host == "" {
		if Settings.Inventory.TLS.Enabled {
			url.Scheme = "https"
		} else {
			url.Scheme = "http"
		}
		url.Host = c.Host
	}

	return url.String()
}

//
// Http GET
func (c *RestClient) get(path string, resource interface{}) (status int, err error) {
	err = c.buildTransport()
	if err != nil {
		return
	}
	c.buildHeader()
	url := c.url(path)
	status, err = c.LibClient.Get(url, resource)
	return
}
