package ova

import (
	"fmt"
	"net"
	"net/http"
	liburl "net/url"
	"time"

	"github.com/go-logr/logr"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
	core "k8s.io/api/core/v1"
)

// Not found error.
type NotFound struct {
}

func (e *NotFound) Error() string {
	return "not found."
}

// Client.
type Client struct {
	URL        string
	client     *libweb.Client
	Secret     *core.Secret
	Log        logr.Logger
	serviceURL string
}

// Connect.
func (r *Client) Connect(provider *api.Provider) (err error) {

	if r.client != nil {
		return
	}

	client := &libweb.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 15 * time.Second,
			}).DialContext,
			MaxIdleConns: 10,
		},
	}

	serviceURL := fmt.Sprintf("http://ova-service-%s:8080", provider.Name)
	if serviceURL == "" {
		return
	}

	url := serviceURL + "/test_connection"
	res := ""
	status, err := client.Get(url, &res)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	r.client = client
	r.serviceURL = serviceURL
	return
}

// List collection.
func (r *Client) list(path string, list interface{}) (err error) {
	url, err := liburl.Parse(r.serviceURL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	url.Path += "/" + path
	status, err := r.client.Get(url.String(), list)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	return
}

// Get a resource.
func (r *Client) get(path string, object interface{}) (err error) {
	url, err := liburl.Parse(r.serviceURL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	url.Path = path
	defer func() {
		if err != nil {
			err = liberr.Wrap(err, "url", url.String())
		}
	}()
	status, err := r.client.Get(url.String(), object)
	if err != nil {
		return
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		err = &NotFound{}
	default:
		err = liberr.New(http.StatusText(status))
	}

	return
}
