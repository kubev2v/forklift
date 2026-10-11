/*
 Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package pmax

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dell/csmlog"
	"github.com/dell/gopowermax/v2/api"
	v100 "github.com/dell/gopowermax/v2/types/v100"
)

// Client is the callers handle to the pmax client library.
// Obtain a client by calling NewClient.
type Client struct {
	configConnect  *ConfigConnect
	api            api.Client
	allowedArrays  []string
	version        string
	symmetrixID    string
	contextTimeout time.Duration
	headers        clientHeaders
}

type clientHeaders struct {
	accept          string
	contentType     string
	applicationType string
}

var (
	errNilReponse = errors.New("nil response from API")
	errBodyRead   = errors.New("error reading body")
	errNoLink     = errors.New("Error: problem finding link")
	debug, _      = strconv.ParseBool(os.Getenv("X_CSI_POWERMAX_DEBUG"))
	// PmaxTimeout is the timeout value for pmax calls.
	// If Unisphere fails to answer within this period, an error will be returned.
	defaultPmaxTimeout = 10 * time.Minute
)

// Authenticate and get API version
func (c *Client) Authenticate(ctx context.Context, configConnect *ConfigConnect) error {
	if debug {
		csmlog.Debug(fmt.Sprintf("PowerMax debug: %v", debug))
	}

	// Store explicit version before assignment to track if user requested specific version
	explicitVersion := configConnect.Version
	c.configConnect = configConnect
	if configConnect.Version != "" {
		c.version = configConnect.Version
	}
	c.api.SetToken("")
	basicAuthString := basicAuth(configConnect.Username, configConnect.Password)

	headers := make(map[string]string, 1)
	headers["Authorization"] = "Basic " + basicAuthString
	path := "univmax/restapi/" + "version"
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, path, headers, nil)
	if err != nil {
		csmlog.Error("Failed to get response: " + err.Error())
		return err
	}

	// parse the response
	switch {
	case resp == nil:
		return errNilReponse
	case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
		return c.api.ParseJSONError(resp)
	}

	// Parse version response to extract API version
	versionDetails := &v100.VersionDetails{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(versionDetails); err != nil && err != io.EOF {
		return err
	}
	if versionDetails.APIVersion != "" {
		// If explicit version was provided in ConfigConnect (e.g., "104"), keep it
		// Otherwise, use DefaultAPIVersion for general operations
		if explicitVersion == "" {
			c.version = DefaultAPIVersion
			c.configConnect.Version = DefaultAPIVersion
			csmlog.Debug(fmt.Sprintf("Detected array version: %s, using API version: %s", versionDetails.APIVersion, DefaultAPIVersion))
		} else {
			// Keep the explicit version that was already set
			csmlog.Debug(fmt.Sprintf("Detected array version: %s, using explicit API version: %s", versionDetails.APIVersion, c.version))
		}
		acceptHeader := fmt.Sprintf("%s;version=%s", api.HeaderValContentTypeJSON, c.version)
		c.headers.accept = acceptHeader
		c.headers.contentType = acceptHeader
	}
	csmlog.Info("authentication successful")
	err = resp.Body.Close()
	if err != nil {
		return err
	}
	return nil
}

// GetTimeoutContext sets up a timeout of time PmaxTimeout for the returned context.
// The user caller should call the cancel function that is returned.
func (c *Client) GetTimeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, c.contextTimeout)
	return ctx, cancel
}

// Generate the base 64 Authorization string from username / password
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// NewClient returns a new Client, which is of interface type Pmax.
// The Client holds state for the connection.
// Thhe following environment variables define the connection:
//
//	CSI_POWERMAX_ENDPOINT - A URL of the form https://1.2.3.4:8443
//	CSI_POWERMAX_VERSION - should not be used. Defines a particular form of versioning.
//	CSI_APPLICATION_NAME - Application name which will be used for registering the application with Unisphere REST APIs
//	CSI_POWERMAX_INSECURE - A boolean indicating whether unvalidated certificates can be accepted. Defaults to true.
//	CSI_POWERMAX_USECERTS - Indicates whether to use certificates at all. Defaults to true.
func NewClient() (client Pmax, err error) {
	return NewClientWithArgs(
		os.Getenv("CSI_POWERMAX_ENDPOINT"),
		os.Getenv("CSI_APPLICATION_NAME"),
		os.Getenv("CSI_POWERMAX_INSECURE") == "true",
		os.Getenv("CSI_POWERMAX_USECERTS") == "true",
		"")
}

// NewClientWithArgs allows the user to specify the endpoint, version, application name, insecure boolean, and useCerts boolean
// as direct arguments rather than receiving them from the enviornment. See NewClient().
func NewClientWithArgs(
	endpoint string,
	applicationName string,
	insecure,
	useCerts bool,
	certFile string,
) (client Pmax, err error) {
	contextTimeout := defaultPmaxTimeout
	if timeoutStr := os.Getenv("X_CSI_UNISPHERE_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err != nil {
			csmlog.Error("Unable to parse Unisphere timeout: " + err.Error())
		} else {
			contextTimeout = timeout
		}
	}

	fields := map[string]interface{}{
		"endpoint":        endpoint,
		"applicationName": applicationName,
		"insecure":        insecure,
		"useCerts":        useCerts,
		"version":         DefaultAPIVersion,
		"debug":           debug,
	}

	csmlog.WithFields(fields).Debug("pmax client init")

	if endpoint == "" {
		csmlog.WithFields(fields).Error("endpoint is required")
		return nil, fmt.Errorf("Endpoint must be supplied, e.g. https://1.2.3.4:8443")
	}

	opts := api.ClientOptions{
		Insecure: insecure,
		UseCerts: useCerts,
		ShowHTTP: debug,
		CertFile: certFile,
	}

	ac, err := api.New(endpoint, opts, debug)
	if err != nil {
		csmlog.Error("Unable to create HTTP client: " + err.Error())
		return nil, err
	}

	acceptHeader := fmt.Sprintf("%s;version=%s", api.HeaderValContentTypeJSON, DefaultAPIVersion)

	client = &Client{
		api: ac,
		configConnect: &ConfigConnect{
			Version: DefaultAPIVersion,
		},
		allowedArrays:  []string{},
		version:        DefaultAPIVersion,
		contextTimeout: contextTimeout,
		headers: clientHeaders{
			accept:          acceptHeader,
			contentType:     acceptHeader,
			applicationType: applicationName,
		},
	}

	return client, nil
}

// WithSymmetrixID sets the default array for the client
func (c *Client) WithSymmetrixID(symmetrixID string) Pmax {
	client := *c
	client.symmetrixID = symmetrixID
	return &client
}

// SetContextTimeout sets the context timeout value for the API requests
func (c *Client) SetContextTimeout(timeout time.Duration) Pmax {
	c.contextTimeout = timeout
	return c
}

func (c *Client) getDefaultHeaders() map[string]string {
	headers := make(map[string]string)
	headers["Accept"] = c.headers.accept
	if c.headers.applicationType != "" {
		headers["Application-Type"] = c.headers.applicationType
	}
	headers["Content-Type"] = c.headers.contentType
	basicAuthString := basicAuth(c.configConnect.Username, c.configConnect.Password)
	headers["Authorization"] = "Basic " + basicAuthString
	if c.symmetrixID != "" {
		headers["symid"] = c.symmetrixID
	}
	return headers
}

// GetHTTPClient will return an underlying http client
func (c *Client) GetHTTPClient() *http.Client {
	return c.api.GetHTTPClient()
}

// SetToken sets the Auth token for the HTTP client
func (c *Client) SetToken(token string) {
	c.api.SetToken(token)
}

// SetCustomHTTPHeaders sets custom HTTP headers that will be sent with every request
func (c *Client) SetCustomHTTPHeaders(headers http.Header) {
	c.api.SetCustomHTTPHeaders(headers)
}

// GetCustomHTTPHeaders returns the current custom HTTP headers
func (c *Client) GetCustomHTTPHeaders() http.Header {
	return c.api.GetCustomHTTPHeaders()
}

// SetRequestObserver registers the observer with the underlying API client.
func (c *Client) SetRequestObserver(observer api.RequestObserver) {
	c.api.SetRequestObserver(observer)
}
