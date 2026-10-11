// Copyright (c) 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dell/csmlog"
	types "github.com/dell/goscaleio/types/v1"
)

const (
	// HeaderKeyAccept is key for  Accept
	HeaderKeyAccept = "Accept"
	// HeaderKeyContentType is key for Content-Type
	HeaderKeyContentType = "Content-Type"
	// HeaderValContentTypeJSON is key for application/json
	HeaderValContentTypeJSON = "application/json"
	// headerValContentTypeBinaryOctetStream is key for binary/octet-stream
	headerValContentTypeBinaryOctetStream = "binary/octet-stream"

	// BearerAuthenticationMinVersion is the minimum version of PowerFlex that supports Bearer authentication
	BearerAuthenticationMinVersion = 4.0
)

var (
	errNewClient = errors.New("missing endpoint")
	errSysCerts  = errors.New("Unable to initialize cert pool from system")
)

// Client is an API client.
type Client interface {
	// Do sends an HTTP request to the API.
	Do(
		ctx context.Context,
		method, path string,
		body, resp interface{}) error

	// DoWithHeaders sends an HTTP request to the API.
	DoWithHeaders(
		ctx context.Context,
		method, path string,
		headers map[string]string,
		body, resp interface{}, version string) error

	// DoandGetREsponseBody sends an HTTP reqeust to the API and returns
	// the raw response body
	DoAndGetResponseBody(
		ctx context.Context,
		method, path string,
		headers map[string]string,
		body interface{}, version string) (*http.Response, error)

	// Get sends an HTTP request using the GET method to the API.
	Get(
		ctx context.Context,
		path string,
		headers map[string]string,
		resp interface{}) error

	// Post sends an HTTP request using the POST method to the API.
	Post(
		ctx context.Context,
		path string,
		headers map[string]string,
		body, resp interface{}) error

	// Put sends an HTTP request using the PUT method to the API.
	Put(
		ctx context.Context,
		path string,
		headers map[string]string,
		body, resp interface{}) error

	// Delete sends an HTTP request using the DELETE method to the API.
	Delete(
		ctx context.Context,
		path string,
		headers map[string]string,
		resp interface{}) error

	// SetToken sets the Auth token for the HTTP client
	SetToken(token string)

	// GetToken gets the Auth token for the HTTP client
	GetToken() string

	// ParseJSONError parses the JSON in r into an error object
	ParseJSONError(r *http.Response) error

	// DoXMLRequest sends an HTTP request to the API.
	DoXMLRequest(
		ctx context.Context,
		method, path, version string,
		body, response interface{},
	) (*http.Response, error)

	// SetCustomHTTPHeaders sets custom HTTP headers that will be sent with every request
	SetCustomHTTPHeaders(headers http.Header)

	// GetCustomHTTPHeaders returns the current custom HTTP headers
	GetCustomHTTPHeaders() http.Header

	// SetRequestObserver sets an observer that is notified for every API request.
	SetRequestObserver(observer RequestObserver)
}

// RequestObservation describes a single API request observed by the client.
type RequestObservation struct {
	Endpoint   string
	Method     string
	StatusCode int
	Duration   time.Duration
	Err        error
}

// RequestObserver receives request observations without importing metrics code.
// Observations may be called concurrently from multiple goroutines.
type RequestObserver interface {
	ObserveRequest(RequestObservation)
}

type SafeHeader struct {
	mu     *sync.RWMutex
	header http.Header
}

func NewSafeHeader() *SafeHeader {
	return &SafeHeader{
		mu:     &sync.RWMutex{},
		header: make(http.Header),
	}
}

func (s *SafeHeader) SetHeader(h http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.header = h.Clone() // clone to avoid external mutations
}

func (s *SafeHeader) GetHeader() http.Header {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h := s.header.Clone()
	return h // return a safe copy
}

func (c *client) SetRequestObserver(observer RequestObserver) {
	c.requestObserver = observer
}

func (c *client) observeRequest(endpoint, method string, start time.Time, statusCode int, callErr error) {
	if c == nil || c.requestObserver == nil {
		return
	}

	observation := RequestObservation{
		Endpoint:   endpoint,
		Method:     method,
		StatusCode: statusCode,
		Duration:   time.Since(start),
		Err:        callErr,
	}

	defer func() {
		if r := recover(); r != nil {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "ObserveRequest",
			}).Errorf("RequestObserver panic: %v", r)
		}
	}()

	c.requestObserver.ObserveRequest(observation)
}

type client struct {
	http              *http.Client
	host              string
	token             string
	customHTTPHeaders *SafeHeader
	requestObserver   RequestObserver
}

// GetSecuredCipherSuites returns a slice of secured cipher suites.
// It iterates over the tls.CipherSuites() and appends the ID of each cipher su                                                                             ite to the suites slice.
// The function returns the suites slice.
func GetSecuredCipherSuites() (suites []uint16) {
	securedSuite := tls.CipherSuites()
	for _, v := range securedSuite {
		suites = append(suites, v.ID)
	}
	return suites
}

// ClientOptions are options for the API client.
type ClientOptions struct {
	// Insecure is a flag that indicates whether or not to supress SSL errors.
	Insecure bool

	// UseCerts is a flag that indicates whether system certs should be loaded
	UseCerts bool

	// CAFilePath is the path to a custom CA certificate file
	CAFilePath string

	// Timeout specifies a time limit for requests made by this client.
	Timeout time.Duration
}

// New returns a new API client.
func New(
	_ context.Context,
	host string,
	opts ClientOptions,
) (Client, error) {
	if host == "" {
		return nil, errNewClient
	}

	host = strings.Replace(host, "/api", "", 1)

	c := &client{
		http:              &http.Client{},
		host:              host,
		customHTTPHeaders: NewSafeHeader(),
	}

	if opts.Timeout != 0 {
		c.http.Timeout = opts.Timeout
	}

	if opts.Insecure {
		c.http.Transport = &http.Transport{
			// #nosec G402
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true, // #nosec G402
				CipherSuites:       GetSecuredCipherSuites(),
			},
		}
	}

	if !opts.Insecure || opts.UseCerts {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, errSysCerts
		}

		// Load custom CA certificate if provided
		if opts.CAFilePath != "" {
			// Open a restricted view rooted at the file's directory, then open the file by its base name.
			dir := filepath.Dir(opts.CAFilePath)
			base := filepath.Base(opts.CAFilePath)

			root, err := os.OpenRoot(dir)
			if err != nil {
				return nil, fmt.Errorf("unable to read rootCA file %q: %v", opts.CAFilePath, err)
			}

			file, err := root.Open(base)
			if err != nil {
				return nil, fmt.Errorf("unable to read rootCA file %q: %v", opts.CAFilePath, err)
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				return nil, fmt.Errorf("unable to read rootCA file %q: %v", opts.CAFilePath, err)
			}

			block, _ := pem.Decode(data)
			if block == nil || block.Type != "CERTIFICATE" {
				return nil, fmt.Errorf("failed to decode PEM block containing certificate")
			}

			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse certificate: %v", err)
			}

			if !cert.IsCA {
				return nil, fmt.Errorf("%s is not a CA", opts.CAFilePath)
			}

			ok := pool.AppendCertsFromPEM(data)
			if !ok {
				return nil, fmt.Errorf("failed to append CA certificate from file: %s", opts.CAFilePath)
			}
		}

		c.http.Transport = &http.Transport{
			// #nosec G402
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				RootCAs:            pool,
				InsecureSkipVerify: opts.Insecure,
				CipherSuites:       GetSecuredCipherSuites(),
			},
		}
	}

	return c, nil
}

func (c *client) Get(
	ctx context.Context,
	path string,
	headers map[string]string,
	resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodGet, path, headers, nil, resp, "",
	)
}

func (c *client) Post(
	ctx context.Context,
	path string,
	headers map[string]string,
	body, resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodPost, path, headers, body, resp, "",
	)
}

func (c *client) Put(
	ctx context.Context,
	path string,
	headers map[string]string,
	body, resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodPut, path, headers, body, resp, "",
	)
}

func (c *client) Delete(
	ctx context.Context,
	path string,
	headers map[string]string,
	resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodDelete, path, headers, nil, resp, "",
	)
}

func (c *client) Do(
	ctx context.Context,
	method, path string,
	body, resp interface{},
) error {
	return c.DoWithHeaders(ctx, method, path, nil, body, resp, "")
}

func beginsWithSlash(s string) bool {
	return s[0] == '/'
}

func endsWithSlash(s string) bool {
	return s[len(s)-1] == '/'
}

func (c *client) DoWithHeaders(
	ctx context.Context,
	method, uri string,
	headers map[string]string,
	body, resp interface{}, version string,
) error {
	res, err := c.DoAndGetResponseBody(
		ctx, method, uri, headers, body, version,
	)
	if err != nil {
		return err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "DoWithHeaders",
			}).Errorf("Failed to close response body: %v", err)
		}
	}()

	// parse the response
	switch {
	case res == nil:
		return nil
	case res.StatusCode >= 200 && res.StatusCode <= 299:
		if resp == nil {
			return nil
		}
		dec := json.NewDecoder(res.Body)
		if err = dec.Decode(resp); err != nil && err != io.EOF {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "DoWithHeaders",
			}).Errorf("unable to decode response body: %v", err)
			return err
		}
	default:
		return c.ParseJSONError(res)
	}

	return nil
}

func (c *client) DoAndGetResponseBody(
	ctx context.Context,
	method, uri string,
	headers map[string]string,
	body interface{}, version string,
) (*http.Response, error) {
	var (
		err                error
		req                *http.Request
		res                *http.Response
		start              = time.Now()
		ubf                = &bytes.Buffer{}
		luri               = len(uri)
		hostEndsWithSlash  = endsWithSlash(c.host)
		uriBeginsWithSlash = beginsWithSlash(uri)
	)

	ubf.WriteString(c.host)

	if !hostEndsWithSlash && (luri > 0) {
		ubf.WriteString("/")
	}

	if luri > 0 {
		if uriBeginsWithSlash {
			ubf.WriteString(uri[1:])
		} else {
			ubf.WriteString(uri)
		}
	}

	u, err := url.Parse(ubf.String())
	if err != nil {
		c.observeRequest(uri, method, start, 0, err)
		return nil, err
	}

	var isContentTypeSet bool

	// marshal the message body (assumes json format)
	if r, ok := body.(io.ReadCloser); ok {
		req, err = http.NewRequest(method, u.String(), r)
		if err != nil {
			c.observeRequest(uri, method, start, 0, err)
			return nil, err
		}

		defer func() {
			if err := r.Close(); err != nil {
				csmlog.WithFields(csmlog.Fields{
					csmlog.FieldComponent: "goscaleio",
					csmlog.FieldOperation: "DoAndGetResponseBody",
				}).Errorf("Failed to close request body: %v", err)
			}
		}()

		if v, ok := headers[HeaderKeyContentType]; ok {
			req.Header.Set(HeaderKeyContentType, v)
		} else {
			req.Header.Set(
				HeaderKeyContentType, headerValContentTypeBinaryOctetStream,
			)
		}
		isContentTypeSet = true
	} else if body != nil {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		if err = enc.Encode(body); err != nil {
			c.observeRequest(uri, method, start, 0, err)
			return nil, err
		}
		req, err = http.NewRequest(method, u.String(), buf)
		if err != nil {
			c.observeRequest(uri, method, start, 0, err)
			return nil, err
		}
		if v, ok := headers[HeaderKeyContentType]; ok {
			req.Header.Set(HeaderKeyContentType, v)
		} else {
			req.Header.Set(HeaderKeyContentType, HeaderValContentTypeJSON)
		}
		isContentTypeSet = true
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
		if err != nil {
			c.observeRequest(uri, method, start, 0, err)
			return nil, err
		}
	}

	if err != nil {
		c.observeRequest(uri, method, start, 0, err)
		return nil, err
	}

	if !isContentTypeSet {
		isContentTypeSet = req.Header.Get(HeaderKeyContentType) != ""
	}

	// add headers to the request
	for header, value := range headers {
		if header == HeaderKeyContentType && isContentTypeSet {
			continue
		}
		req.Header.Add(header, value)
	}

	if version != "" {
		ver, err := strconv.ParseFloat(version, 64)
		if err != nil {
			c.observeRequest(uri, method, start, 0, err)
			return nil, err
		}

		// set the auth token
		if c.token != "" {
			// use Bearer Authentication if the powerflex array
			// version >= 4.0
			if ver >= BearerAuthenticationMinVersion {
				bearer := "Bearer " + c.token
				req.Header.Set("Authorization", bearer)
			} else {
				req.SetBasicAuth("", c.token)
			}
		}

	} else {
		if c.token != "" {
			req.SetBasicAuth("", c.token)
		}
	}

	for key, values := range c.customHTTPHeaders.GetHeader() {
		for _, elem := range values {
			req.Header.Add(key, elem)
		}
	}

	logRequest(ctx, req)

	// send the request
	req = req.WithContext(ctx)
	if res, err = c.http.Do(req); err != nil { // #nosec G704 - Request to user-provided gateway endpoint. This is intended SDK behavior where users configure their own gateway URL
		c.observeRequest(uri, method, start, 0, err)
		return nil, err
	}

	logResponse(ctx, res)

	statusCode := 0
	if res != nil {
		statusCode = res.StatusCode
	}
	c.observeRequest(uri, method, start, statusCode, nil)

	return res, err
}

func (c *client) SetToken(token string) {
	c.token = token
}

func (c *client) GetToken() string {
	return c.token
}

func (c *client) DoXMLRequest(
	ctx context.Context,
	method, path, version string,
	body, resp interface{},
) (*http.Response, error) {
	var (
		err                error
		req                *http.Request
		res                *http.Response
		start              = time.Now()
		ubf                = &bytes.Buffer{}
		luri               = len(path)
		hostEndsWithSlash  = endsWithSlash(c.host)
		uriBeginsWithSlash = beginsWithSlash(path)
	)
	ubf.WriteString(c.host)

	if !hostEndsWithSlash && (luri > 0) {
		ubf.WriteString("/")
	}

	if luri > 0 {
		if uriBeginsWithSlash {
			ubf.WriteString(path[1:])
		} else {
			ubf.WriteString(path)
		}
	}

	u, err := url.Parse(ubf.String())
	if err != nil {
		c.observeRequest(path, method, start, 0, err)
		return nil, err
	}
	if body != nil {
		xmlBody, err := xml.Marshal(body)
		if err != nil {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "DoXMLRequest",
			}).Errorf("Failed to marshal XML request body: %v", err)
			c.observeRequest(path, method, start, 0, err)
			return nil, err
		}

		// Create the HTTP request
		req, err = http.NewRequest(method, u.String(), bytes.NewBuffer(xmlBody))
		if err != nil {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "DoXMLRequest",
				"http_method":         method,
				"request_path":        path,
			}).Errorf("Failed to create XML request: %v", err)
			c.observeRequest(path, method, start, 0, err)
			return nil, err
		}
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
		if err != nil {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "DoXMLRequest",
				"http_method":         method,
				"request_path":        path,
			}).Errorf("Failed to create XML request: %v", err)
			c.observeRequest(path, method, start, 0, err)
			return nil, err
		}
	}

	req.Header.Set("Content-Type", "application/xml")
	// add headers to the request
	if version != "" {
		ver, err := strconv.ParseFloat(version, 64)
		if err != nil {
			c.observeRequest(path, method, start, 0, err)
			return nil, err
		}

		// set the auth token
		if c.token != "" {
			// use Bearer Authentication if the powerflex array
			// version >= 4.0
			if ver >= 4.0 {
				bearer := "Bearer " + c.token
				req.Header.Set("Authorization", bearer)
			} else {
				req.SetBasicAuth("", c.token)
			}
		}

	} else {
		if c.token != "" {
			req.SetBasicAuth("", c.token)
		}
	}

	// send the request
	req = req.WithContext(ctx)
	if res, err = c.http.Do(req); err != nil { // #nosec G704 - Request to user-provided gateway endpoint. This is intended SDK behavior where users configure their own gateway URL
		c.observeRequest(path, method, start, 0, err)
		return nil, err
	}

	// parse the response
	switch {
	case res == nil:
		return nil, nil
	case res.StatusCode >= 200 && res.StatusCode <= 299:
		if resp == nil {
			return nil, nil
		}
		dec := json.NewDecoder(res.Body)
		if err = dec.Decode(resp); err != nil && err != io.EOF {
			csmlog.WithFields(csmlog.Fields{
				csmlog.FieldComponent: "goscaleio",
				csmlog.FieldOperation: "DoXMLRequest",
				"http_method":         method,
				"request_path":        path,
			}).Errorf("Unable to decode XML response body: %v", err)
			c.observeRequest(path, method, start, res.StatusCode, err)
			return nil, err
		}
	default:
		err = c.ParseJSONError(res)
		c.observeRequest(path, method, start, res.StatusCode, err)
		return nil, err
	}

	c.observeRequest(path, method, start, res.StatusCode, nil)

	return res, err
}

func (c *client) ParseJSONError(r *http.Response) error {
	jsonError := &types.Error{}

	// Starting in 4.0, response may be in html; so we cannot always use a json decoder
	if strings.Contains(r.Header.Get("Content-Type"), "html") {
		jsonError.HTTPStatusCode = r.StatusCode
		jsonError.Message = r.Status
		return jsonError
	}

	if err := json.NewDecoder(r.Body).Decode(jsonError); err != nil {
		return err
	}

	jsonError.HTTPStatusCode = r.StatusCode
	if jsonError.Message == "" {
		jsonError.Message = r.Status
	}

	return jsonError
}

// SetCustomHTTPHeaders method register headers which will be sent with every request
func (c *client) SetCustomHTTPHeaders(headers http.Header) {
	c.customHTTPHeaders.SetHeader(headers)
}

// GetCustomHTTPHeaders method retrieves http headers
func (c *client) GetCustomHTTPHeaders() http.Header {
	return c.customHTTPHeaders.GetHeader()
}
