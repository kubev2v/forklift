// Copyright © 2019 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
)

var (
	errNewClient = errors.New("missing endpoint")
	errSysCerts  = errors.New("Unable to initialize cert pool from system")
	logger       = slog.New(slog.NewTextHandler(os.Stderr, nil))
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
}

type client struct {
	http     *http.Client
	host     string
	token    string
	showHTTP bool
	debug    bool
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

	// Timeout specifies a time limit for requests made by this client.
	Timeout time.Duration

	// ShowHTTP is a flag that indicates whether or not HTTP requests and
	// responses should be logged to stdout
	ShowHTTP bool
}

// New returns a new API client.
func New(
	_ context.Context,
	host string,
	opts ClientOptions,
	debug bool,
) (Client, error) {
	if host == "" {
		return nil, errNewClient
	}

	host = strings.Replace(host, "/api", "", 1)

	c := &client{
		http: &http.Client{},
		host: host,
	}

	if opts.Timeout != 0 {
		c.http.Timeout = opts.Timeout
	}

	if opts.Insecure {
		c.http.Transport = &http.Transport{
			// #nosec G402
			TLSClientConfig: &tls.Config{
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

		c.http.Transport = &http.Transport{
			// #nosec G402
			TLSClientConfig: &tls.Config{
				RootCAs:            pool,
				InsecureSkipVerify: opts.Insecure,
				CipherSuites:       GetSecuredCipherSuites(),
			},
		}
	}

	if opts.ShowHTTP {
		c.showHTTP = true
	}

	c.debug = debug

	return c, nil
}

func (c *client) Get(
	ctx context.Context,
	path string,
	headers map[string]string,
	resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodGet, path, headers, nil, resp, "")
}

func (c *client) Post(
	ctx context.Context,
	path string,
	headers map[string]string,
	body, resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodPost, path, headers, body, resp, "")
}

func (c *client) Put(
	ctx context.Context,
	path string,
	headers map[string]string,
	body, resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodPut, path, headers, body, resp, "")
}

func (c *client) Delete(
	ctx context.Context,
	path string,
	headers map[string]string,
	resp interface{},
) error {
	return c.DoWithHeaders(
		ctx, http.MethodDelete, path, headers, nil, resp, "")
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
		ctx, method, uri, headers, body, version)
	if err != nil {
		return err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			c.doLog(logger.Error, err.Error())
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
			c.doLog(logger.Error, fmt.Sprintf("Error: %s Unable to decode response into %+v", err.Error(), resp))
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
		return nil, err
	}

	var isContentTypeSet bool

	// marshal the message body (assumes json format)
	if r, ok := body.(io.ReadCloser); ok {
		req, err = http.NewRequest(method, u.String(), r)

		defer func() {
			if err := r.Close(); err != nil {
				c.doLog(logger.Error, err.Error())
			}
		}()

		if v, ok := headers[HeaderKeyContentType]; ok {
			req.Header.Set(HeaderKeyContentType, v)
		} else {
			req.Header.Set(
				HeaderKeyContentType, headerValContentTypeBinaryOctetStream)
		}
		isContentTypeSet = true
	} else if body != nil {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		if err = enc.Encode(body); err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, u.String(), buf)
		if v, ok := headers[HeaderKeyContentType]; ok {
			req.Header.Set(HeaderKeyContentType, v)
		} else {
			req.Header.Set(HeaderKeyContentType, HeaderValContentTypeJSON)
		}
		isContentTypeSet = true
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}

	if err != nil {
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

	if c.showHTTP {
		logRequest(ctx, req, c.doLog)
	}

	// send the request
	req = req.WithContext(ctx)
	if res, err = c.http.Do(req); err != nil {
		return nil, err
	}

	if c.showHTTP {
		logResponse(ctx, res, c.doLog)
	}

	return res, err
}

func (c *client) SetToken(token string) {
	c.token = token
}

func (c *client) GetToken() string {
	return c.token
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

func (c *client) doLog(
	l func(msg string, args ...any),
	msg string,
) {
	if c.debug {
		l(msg)
	}
}
