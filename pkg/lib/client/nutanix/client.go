// Package nutanix provides a shared Nutanix Prism REST client and wire types
// used by the inventory collector and the migration plan adapter.
package nutanix

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	liburl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kubev2v/forklift/pkg/controller/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
)

// ConnectionTimeout bounds how long Connect() and subsequent requests wait
// for a response once a request has been sent, when the caller hasn't set
// a more specific Client.Timeout.
const ConnectionTimeout = 30 * time.Second

// maxListPages bounds pagination against a server that ignores page
// parameters and returns the same page indefinitely.
var maxListPages = 1000

// Client is a minimal Nutanix Prism (v3/v4) REST client shared by the
// inventory collector and the migration plan adapter. Callers set URL and
// Secret (and optionally Timeout and Log) before calling Connect().
//
// A connected Client is safe for concurrent use by multiple goroutines once
// Connect has succeeded. Connect uses double-checked locking so concurrent
// first callers serialize transport setup; failed connects may be retried.
type Client struct {
	// Base URL (e.g., https://prism-central:9440).
	URL string
	// Secret containing credentials (user/password, ca.crt, insecureSkipVerify).
	Secret *core.Secret
	// Per-request response timeout. Defaults to ConnectionTimeout when zero.
	Timeout time.Duration
	// Logger. Defaults to a package logger when unset.
	Log logging.LevelLogger

	mu     sync.Mutex
	client *libweb.Client
}

// Connect and authenticate with Nutanix Prism. Idempotent: repeated calls
// on an already-connected Client are a no-op. Connectivity is verified with
// a minimal request (listing a single cluster).
func (r *Client) Connect() (status int, err error) {
	if r.client != nil {
		return http.StatusOK, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		return http.StatusOK, nil
	}

	if r.Log == nil {
		r.Log = logging.WithName("client|nutanix")
	}

	var tlsClientConfig *tls.Config
	if base.GetInsecureSkipVerifyFlag(r.Secret) {
		tlsClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cacert, found := util.GetCACert(r.Secret); found {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cacert)
		if !ok {
			err = liberr.New("failed to parse CA certificate")
			return http.StatusBadRequest, err
		}
		tlsClientConfig = &tls.Config{RootCAs: roots}
	} else {
		tlsClientConfig = &tls.Config{InsecureSkipVerify: false}
	}

	r.URL = strings.TrimRight(r.URL, "/")

	responseHeaderTimeout := r.Timeout
	if responseHeaderTimeout <= 0 {
		responseHeaderTimeout = ConnectionTimeout
	}

	r.client = &libweb.Client{
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
			ResponseHeaderTimeout: responseHeaderTimeout,
			TLSClientConfig:       tlsClientConfig,
		},
	}

	status, err = r.testConnection()
	if err != nil {
		r.client = nil
		return status, err
	}

	r.Log.Info("Successfully connected to Nutanix", "url", r.URL)

	return http.StatusOK, nil
}

// testConnection verifies the URL and credentials with a minimal request.
func (r *Client) testConnection() (status int, err error) {
	url := fmt.Sprintf("%s/api/nutanix/v3/clusters/list", r.URL)
	body := v3ListRequest{
		Kind:   "cluster",
		Offset: 0,
		Length: 1,
	}

	status, err = r.send(http.MethodPost, url, body, nil, r.createMutatingHeader())
	if err != nil {
		return status, liberr.Wrap(err, "connection test failed")
	}
	if status != http.StatusOK {
		return status, liberr.New("connection test failed", "status", status)
	}

	return http.StatusOK, nil
}

// Get issues an authenticated GET request.
func (r *Client) Get(url string, out any, params ...libweb.Param) (status int, err error) {
	status, err = r.Connect()
	if err != nil {
		return
	}
	if len(params) > 0 {
		parsedURL, parseErr := liburl.Parse(url)
		if parseErr != nil {
			return 0, liberr.Wrap(parseErr, "URL not valid", "url", url)
		}
		query := parsedURL.Query()
		for _, param := range params {
			query.Add(param.Key, param.Value)
		}
		parsedURL.RawQuery = query.Encode()
		url = parsedURL.String()
	}
	return r.send(http.MethodGet, url, nil, out, r.createAuthHeader())
}

// GetNoRedirect issues an authenticated GET without following redirects,
// returning the raw response status and headers instead of decoding a
// body. This exists for endpoints that hand back a redirect carrying
// caller-specific instructions in its headers (e.g. a token that must be
// replayed as a cookie on the next request) -- something a normal
// redirect-following client can't act on, since it never gets to see
// that intermediate response.
func (r *Client) GetNoRedirect(url string) (status int, header http.Header, err error) {
	status, err = r.Connect()
	if err != nil {
		return
	}

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, liberr.Wrap(err, "failed to build request", "url", url)
	}
	request.Header = r.createAuthHeader()

	client := r.httpClient()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, liberr.Wrap(err, "request failed", "url", url)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	return response.StatusCode, response.Header, nil
}

// Post issues an authenticated POST request. Nutanix v3 uses POST for list
// operations as well as creation. Mutating v4 endpoints require a unique
// NTNX-Request-Id; it is harmless on v3, so every Post carries one.
func (r *Client) Post(url string, body any, out any) (status int, err error) {
	status, err = r.Connect()
	if err != nil {
		return
	}
	return r.send(http.MethodPost, url, body, out, r.createMutatingHeader())
}

// Put issues an authenticated PUT request (Nutanix v3's update pattern:
// GET the current spec, modify it, PUT the full spec back).
func (r *Client) Put(url string, body any, out any) (status int, err error) {
	status, err = r.Connect()
	if err != nil {
		return
	}
	return r.send(http.MethodPut, url, body, out, r.createMutatingHeader())
}

// Delete issues an authenticated DELETE request. `out`, if non-nil,
// receives the decoded response body (Nutanix v3 deletes return a task
// reference).
func (r *Client) Delete(url string, out any) (status int, err error) {
	status, err = r.Connect()
	if err != nil {
		return
	}
	return r.send(http.MethodDelete, url, nil, out, r.createMutatingHeader())
}

// send issues an authenticated HTTP request with an optional JSON (or raw
// string) body and optionally decodes a JSON response into `out`.
func (r *Client) send(method, url string, in, out any, header http.Header) (status int, err error) {
	parsedURL, err := liburl.Parse(url)
	if err != nil {
		return 0, liberr.Wrap(err, "URL not valid", "url", url)
	}
	var bodyReader io.Reader
	if in != nil {
		switch v := in.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		default:
			var marshaled []byte
			marshaled, err = json.Marshal(in)
			if err != nil {
				return 0, liberr.Wrap(err, "json marshal failed", "url", url)
			}
			bodyReader = bytes.NewReader(marshaled)
		}
	}
	request, err := http.NewRequest(method, parsedURL.String(), bodyReader)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to build request", "url", url)
	}
	request.Header = header

	client := r.httpClient()
	response, err := client.Do(request)
	if err != nil {
		return 0, liberr.Wrap(err, method+" failed", "url", url)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, liberr.Wrap(err, "read body failed", "url", url)
	}
	status = response.StatusCode
	if status >= http.StatusOK && status < http.StatusMultipleChoices && out != nil && len(content) > 0 {
		if err = json.Unmarshal(content, out); err != nil {
			return status, liberr.Wrap(err, "json unmarshal failed", "url", url)
		}
	}
	return status, nil
}

// ListV3 resources using the Nutanix v3 API pattern: POST with an
// offset/length body. filter is optional (nil omits the filter field).
func ListV3[T any](r *Client, resourceKind string, offset, length int, filter map[string]any) (result V3ListResponse[T], err error) {
	url := fmt.Sprintf("%s/api/nutanix/v3/%ss/list", r.URL, resourceKind)
	var body any
	if filter != nil {
		body = v3ListRequestWithFilter{
			Kind:   resourceKind,
			Offset: offset,
			Length: length,
			Filter: filter,
		}
	} else {
		body = v3ListRequest{
			Kind:   resourceKind,
			Offset: offset,
			Length: length,
		}
	}

	status, err := r.Post(url, body, &result)
	if err != nil {
		return V3ListResponse[T]{}, err
	}
	if status != http.StatusOK {
		return V3ListResponse[T]{}, liberr.New(fmt.Sprintf("unexpected status: %d", status))
	}

	return result, nil
}

// ListAllV3 pages through a v3 list endpoint, following total_matches.
// filter is optional (nil omits the filter field).
func ListAllV3[T any](r *Client, resourceKind string, pageSize int, filter map[string]any) (entities []T, err error) {
	offset := 0
	pages := 0
	entities = make([]T, 0)

	for {
		if pages >= maxListPages {
			return nil, liberr.New("pagination limit exceeded", "resource", resourceKind, "pages", pages)
		}
		pages++

		result, err := ListV3[T](r, resourceKind, offset, pageSize, filter)
		if err != nil {
			return nil, err
		}

		entities = append(entities, result.Entities...)

		if len(result.Entities) == 0 {
			break
		}

		offset += len(result.Entities)
		if offset >= result.Metadata.TotalMatches {
			break
		}
	}

	return entities, nil
}

// ListAllV4 pages through a v4 list endpoint, following
// metadata.totalAvailableResults. v4 endpoints paginate via $page/$limit
// query params rather than v3's offset/length body fields.
func ListAllV4[T any](r *Client, path string, pageSize int) (entities []T, err error) {
	url := fmt.Sprintf("%s%s", r.URL, path)
	page := 0
	entities = make([]T, 0)

	for {
		if page >= maxListPages {
			return nil, liberr.New("pagination limit exceeded", "path", path, "pages", page)
		}

		var result V4ListResponse[T]
		status, err := r.Get(url, &result,
			libweb.Param{Key: "$page", Value: strconv.Itoa(page)},
			libweb.Param{Key: "$limit", Value: strconv.Itoa(pageSize)},
		)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, liberr.New(fmt.Sprintf("unexpected status listing %s: %d", path, status))
		}

		entities = append(entities, result.Data...)

		if len(result.Data) == 0 {
			break
		}

		if len(entities) >= result.Metadata.TotalAvailableResults {
			break
		}

		page++
	}

	return entities, nil
}

func (r *Client) httpClient() http.Client {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = ConnectionTimeout
	}
	return http.Client{
		Transport: r.client.Transport,
		Timeout:   timeout,
	}
}

// createAuthHeader builds a Basic Auth header from Secret's user/password.
func (r *Client) createAuthHeader() http.Header {
	user := string(r.Secret.Data["user"])
	password := string(r.Secret.Data["password"])

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", "Basic "+basicAuth(user, password))

	return header
}

// createMutatingHeader is createAuthHeader plus a fresh NTNX-Request-Id.
// Prism Central's v4 mutating APIs reject requests that omit it
// ("Failed to perform the operation as the request ID is missing.").
func (r *Client) createMutatingHeader() http.Header {
	header := r.createAuthHeader()
	header.Set("NTNX-Request-Id", uuid.NewString())
	return header
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
