package ovirt

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	liburl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/controller/base"
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
	// Base URL.
	url string
	// Raw client.
	client *libweb.Client
	// Secret.
	secret                *core.Secret
	clientExpiration      time.Time
	clientTimeout         time.Duration
	accessTokenExpiration time.Time
	log                   logging.LevelLogger
}

type ovirtTokenResponse struct {
	AccessToken string `json:"access_token"`
	Expiration  string `json:"exp"`
}

// Connect.
func (r *Client) connect() (status int, err error) {
	var TLSClientConfig *tls.Config

	if !r.clientExpiration.IsZero() && time.Now().After(r.clientExpiration) {
		r.log.Info("Recreating client, timeout exceeded")
		r.client = nil
	}

	if !r.accessTokenExpiration.IsZero() && time.Now().After(r.accessTokenExpiration) {
		r.log.Info("Recreating client, token expired")
		r.client = nil
	}

	if r.client != nil {
		return
	}

	if base.GetInsecureSkipVerifyFlag(r.secret) {
		TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		cacert := r.secret.Data["cacert"]
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cacert)
		if !ok {
			err = liberr.New("failed to parse cacert")
			return
		}
		TLSClientConfig = &tls.Config{RootCAs: roots}
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
			TLSClientConfig:       TLSClientConfig,
		},
	}

	url, err := liburl.Parse(r.url)
	if err != nil {
		return
	}

	url.Path = "/ovirt-engine/sso/oauth/token"
	values := liburl.Values{}
	values.Add("grant_type", "password")
	values.Add("username", string(r.secret.Data["user"]))
	values.Add("password", string(r.secret.Data["password"]))
	values.Add("scope", "ovirt-app-api")

	client.Header = http.Header{
		"Accept":       []string{"application/json"},
		"Content-Type": []string{"application/x-www-form-urlencoded"},
	}
	response := &ovirtTokenResponse{}
	status, err = client.Post(url.String(), values.Encode(), response)
	if err != nil {
		return
	}

	// Providing bad credentials when requesting the token results
	// in 400, and not 401. So checking for != 200 instead
	if status != http.StatusOK {
		err = liberr.New("Request for token failed", "status", status)
		return
	}

	// Set the access token we received
	client.Header = http.Header{
		"Accept":        []string{"application/json"},
		"Authorization": []string{"Bearer " + response.AccessToken},
		"Version":       []string{"4"},
	}

	r.client = client
	r.clientExpiration = time.Now().Add(r.clientTimeout)

	expiration, err := strconv.ParseInt(response.Expiration, 10, 64)
	if err != nil {
		err = liberr.New("Failed to convert expiration time to integer", "Expiration", response.Expiration)
		return
	}

	r.accessTokenExpiration = time.Now().Local().Add(time.Duration(expiration))

	return
}

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

// Get a resource.
func (r *Client) get(path string, object interface{}, param ...libweb.Param) (err error) {
	url, err := liburl.Parse(r.url)
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
	status, err := r.client.Get(url.String(), object, param...)
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

// Get system.
func (r *Client) system() (system *System, status int, err error) {
	status, err = r.connect()
	if err != nil {
		return
	}
	system = &System{}
	status, err = r.client.Get(r.url, system)
	if err != nil {
		return
	}

	return
}
