package base

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	liburl "net/url"
	"os"
	"reflect"
	"time"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/settings"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Application settings.
var Settings = &settings.Settings

// Lib.
type EventHandler = libweb.EventHandler
type LibClient = libweb.Client
type Watch = libweb.Watch

// Resource kind cannot be resolved.
type ResourceNotResolvedError struct {
	Object interface{}
}

func (r ResourceNotResolvedError) Error() string {
	return fmt.Sprintf("Resource %#v cannot be resolved.", r.Object)
}

// Reference matches multiple resources.
type RefNotUniqueError struct {
	Ref
}

func (r RefNotUniqueError) Error() string {
	return fmt.Sprintf("Reference %#v matched multiple resources.", r.Ref)
}

// Resource not found.
type NotFoundError struct {
	Ref
}

func (r NotFoundError) Error() string {
	return fmt.Sprintf("Resource %#v not found.", r.Ref)
}

// Reference.
type Ref = ref.Ref

// Resolves resources to API paths.
type Resolver interface {
	// Find the API path for the specified resource.
	Path(resource interface{}, id string) (string, error)
}

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

// Web parameter.
type Param struct {
	Key   string
	Value string
}

// REST API client.
type RestClient struct {
	LibClient
	Resolver
	// Host <host>:<port>
	Host string
	// Parameters
	Params Params
}

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

// Build and set the transport as needed.
func (c *RestClient) buildTransport() (err error) {
	if c.Transport != nil {
		return
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if Settings.Inventory.TLS.CA != "" {
		pool := x509.NewCertPool()
		ca, xErr := os.ReadFile(Settings.Inventory.TLS.CA)
		if xErr != nil {
			err = liberr.Wrap(xErr)
			return
		}
		pool.AppendCertsFromPEM(ca)
		transport.TLSClientConfig = &tls.Config{
			RootCAs: pool,
		}
	} else if Settings.Development {
		// Disable TLS for development when certs are missing
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	c.Transport = transport

	return
}

// Build header.
func (c *RestClient) buildHeader() {
	cfg, _ := config.GetConfig()
	c.Header = http.Header{
		"Authorization": []string{
			fmt.Sprintf("Bearer %s", cfg.BearerToken),
		},
	}
}

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
		url.Scheme = "https"
		url.Host = c.Host
	}

	return url.String()
}

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
