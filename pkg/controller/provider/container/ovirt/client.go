package ovirt

import (
	"crypto/tls"
	"encoding/base64"
	liberr "github.com/konveyor/controller/pkg/error"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	core "k8s.io/api/core/v1"
	"net"
	"net/http"
	liburl "net/url"
	"strings"
	"time"
)

//
// Client.
type Client struct {
	// Base URL.
	url string
	// Raw client.
	client *libweb.Client
	// Secret.
	secret *core.Secret
}

//
// Connect.
func (r *Client) connect() (err error) {
	if r.client != nil {
		return
	}
	r.url = strings.TrimRight(r.url, "/")
	client := &libweb.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // TODO:
		},
	}
	client.Header = http.Header{
		"Accept": []string{"application/json"},
		"Authorization": []string{
			"Basic",
			r.auth()},
		"Version": []string{"4"},
	}

	r.client = client

	return
}

//
// List collection.
func (r *Client) list(path string, list interface{}, param ...libweb.Param) (err error) {
	url, err := liburl.Parse(r.url)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	url.Path += "/" + path
	status, err := r.client.Get(url.String(), list, param...)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	return
}

//
// Get a resource.
func (r *Client) get(path string, object interface{}) (err error) {
	url, err := liburl.Parse(r.url)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	url.Path = path
	status, err := r.client.Get(url.String(), object)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	return
}

//
// Basic authorization user.
func (r *Client) auth() (user string) {
	user = strings.Join(
		[]string{
			string(r.secret.Data["user"]),
			string(r.secret.Data["password"]),
		},
		":")

	user = base64.StdEncoding.EncodeToString([]byte(user))

	return
}

//
// Get system.
func (r *Client) system() (s *System, err error) {
	err = r.connect()
	if err != nil {
		return
	}
	system := &System{}
	status, err := r.client.Get(r.url, system)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	return
}
