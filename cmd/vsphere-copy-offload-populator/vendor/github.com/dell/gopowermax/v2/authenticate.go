/*
 Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dell/gopowermax/v2/api"
	log "github.com/sirupsen/logrus"
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
}

var (
	errNilReponse    = errors.New("nil response from API")
	errBodyRead      = errors.New("error reading body")
	errNoLink        = errors.New("Error: problem finding link")
	debug, _         = strconv.ParseBool(os.Getenv("X_CSI_POWERMAX_DEBUG"))
	accHeader        string
	conHeader        string
	applicationType  string
	logResponseTimes bool
	// PmaxTimeout is the timeout value for pmax calls.
	// If Unisphere fails to answer within this period, an error will be returned.
	defaultPmaxTimeout = 10 * time.Minute
)

// Authenticate and get API version
func (c *Client) Authenticate(ctx context.Context, configConnect *ConfigConnect) error {
	if debug {
		log.Printf("PowerMax debug: %v", debug)
		log.SetLevel(log.DebugLevel)
	}

	c.configConnect = configConnect
	c.api.SetToken("")
	basicAuthString := basicAuth(configConnect.Username, configConnect.Password)

	headers := make(map[string]string, 1)
	headers["Authorization"] = "Basic " + basicAuthString
	path := "univmax/restapi/" + "version"
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, path, headers, nil)
	if err != nil {
		doLog(log.WithError(err).Error, "")
		return err
	}

	// parse the response
	switch {
	case resp == nil:
		return errNilReponse
	case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
		return c.api.ParseJSONError(resp)
	}
	doLog(log.Infoln, "authentication successful")
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

func doLog(
	l func(args ...interface{}),
	msg string,
) {
	if debug {
		l(msg)
	}
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
	logResponseTimes, _ = strconv.ParseBool(os.Getenv("X_CSI_POWERMAX_RESPONSE_TIMES"))

	contextTimeout := defaultPmaxTimeout
	if timeoutStr := os.Getenv("X_CSI_UNISPHERE_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err != nil {
			doLog(log.WithError(err).Error, "Unable to parse Unisphere timout")
		} else {
			contextTimeout = timeout
		}
	}

	fields := map[string]interface{}{
		"endpoint":         endpoint,
		"applicationName":  applicationName,
		"insecure":         insecure,
		"useCerts":         useCerts,
		"version":          DefaultAPIVersion,
		"debug":            debug,
		"logResponseTimes": logResponseTimes,
	}

	doLog(log.WithFields(fields).Debug, "pmax client init")

	if endpoint == "" {
		doLog(log.WithFields(fields).Error, "endpoint is required")
		return nil, fmt.Errorf("Endpoint must be supplied, e.g. https://1.2.3.4:8443")
	}

	opts := api.ClientOptions{
		Insecure: insecure,
		UseCerts: useCerts,
		ShowHTTP: debug,
		CertFile: certFile,
	}

	if applicationType != "" {
		log.Debug(fmt.Sprintf("Application type already set to: %s, Resetting it to: %s",
			applicationType, applicationName))
	}
	applicationType = applicationName

	ac, err := api.New(endpoint, opts, debug)
	if err != nil {
		doLog(log.WithError(err).Error, "Unable to create HTTP client")
		return nil, err
	}

	client = &Client{
		api: ac,
		configConnect: &ConfigConnect{
			Version: DefaultAPIVersion,
		},
		allowedArrays:  []string{},
		version:        DefaultAPIVersion,
		contextTimeout: contextTimeout,
	}

	accHeader = api.HeaderValContentTypeJSON
	accHeader = fmt.Sprintf("%s;version=%s", api.HeaderValContentTypeJSON, DefaultAPIVersion)
	conHeader = accHeader

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
	headers["Accept"] = accHeader
	if applicationType != "" {
		headers["Application-Type"] = applicationType
	}
	headers["Content-Type"] = conHeader
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
