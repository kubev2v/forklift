package ova

import (
	"fmt"
	"net"
	"net/http"
	liburl "net/url"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
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
	Log        logging.LevelLogger
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

	service := provider.Status.Service
	svcURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", service.Name, service.Namespace)

	testURL := svcURL + "/test_connection"
	res := ""
	status, err := client.Get(testURL, &res)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	r.client = client
	r.serviceURL = svcURL
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
