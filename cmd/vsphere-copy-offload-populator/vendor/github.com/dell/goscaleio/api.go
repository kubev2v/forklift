// Copyright © 2019 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package goscaleio

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dell/goscaleio/api"
	"github.com/dell/goscaleio/log"
	types "github.com/dell/goscaleio/types/v1"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"golang.org/x/oauth2"
)

var (
	mu        sync.Mutex // guards accHeader and conHeader
	accHeader string
	conHeader string

	errNilResponse = errors.New("nil response from API")
	errBodyRead    = errors.New("error reading body")
	errNoLink      = errors.New("Error: problem finding link")

	debug, _    = strconv.ParseBool(os.Getenv("GOSCALEIO_DEBUG"))
	showHTTP, _ = strconv.ParseBool(os.Getenv("GOSCALEIO_SHOWHTTP"))
)

// Client defines struct for Client
type Client struct {
	ctx           context.Context
	configConnect *ConfigConnect
	api           api.Client
}

// Cluster defines struct for Cluster
type Cluster struct{}

// ConfigConnect defines struct for ConfigConnect

type ConfigConnect struct {
	Endpoint         string
	Version          string
	Username         string
	Password         string
	Insecure         bool
	AuthType         string // "Basic" or "OIDC"
	PfmpIP           string
	CiamClientID     string
	CiamClientSecret string
	OidcClientID     string
	OidcClientSecret string
	Issuer           string
	Scopes           []string
}

// GetVersion returns version
func (c *Client) GetVersion() (string, error) {
	ctx := c.Context()
	defer c.ResetContext()

	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodGet, "/api/version", nil, nil, c.configConnect.Version)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.DoLog(log.Log.Error, err.Error())
		}
	}()
	// parse the response
	switch {
	case resp == nil:
		return "", errNilResponse
	case resp.StatusCode == http.StatusUnauthorized:
		// Authenticate then try again
		if _, err = c.Authenticate(c.configConnect); err != nil {
			return "", err
		}
		resp, err = c.api.DoAndGetResponseBody(
			ctx, http.MethodGet, "/api/version", nil, nil, c.configConnect.Version)
		if err != nil {
			return "", err
		}
	case !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices):
		return "", c.api.ParseJSONError(resp)
	}
	version, err := extractString(resp)
	if err != nil {
		return "", err
	}
	versionRX := regexp.MustCompile(`^(\d+?\.\d+?).*$`)
	if m := versionRX.FindStringSubmatch(version); len(m) > 0 {
		return m[1], nil
	}
	return version, nil
}

// updateVersion updates version
func (c *Client) updateVersion() error {
	version, err := c.GetVersion()
	if err != nil {
		return err
	}
	c.configConnect.Version = version

	updateHeaders(version)

	return nil
}

func updateHeaders(version string) {
	mu.Lock()
	defer mu.Unlock()
	accHeader = api.HeaderValContentTypeJSON
	if version != "" {
		accHeader = accHeader + ";version=" + version
	}
	conHeader = accHeader
}

// Authenticate controls authentication to client
func (c *Client) Authenticate(configConnect *ConfigConnect) (Cluster, error) {
	configConnect.Version = c.configConnect.Version
	c.configConnect = configConnect

	c.api.SetToken("")
	ctx := c.Context()
	defer c.ResetContext()

	var token string
	var err error

	if configConnect.AuthType == "OIDC" {
		log.DoLog(log.Log.Info, "Authenticating with OIDC")
		token, err = c.oidcAuthenticate(configConnect)
		if err != nil {
			return Cluster{}, err
		}
	} else {
		log.DoLog(log.Log.Info, "Basic Authentication")
		token, err = c.basicAuthenticate(ctx, configConnect)
		if err != nil {
			return Cluster{}, err
		}
	}
	c.api.SetToken(token)

	if c.configConnect.Version == "" {
		err = c.updateVersion()
		if err != nil {
			return Cluster{}, errors.New("error getting version of ScaleIO")
		}
	}

	return Cluster{}, nil
}

func (c *Client) basicAuthenticate(ctx context.Context, configConnect *ConfigConnect) (string, error) {
	// Prepare headers for Basic Auth
	headers := map[string]string{
		"Authorization": "Basic " + basicAuth(configConnect.Username, configConnect.Password),
	}

	// Call the login API
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, "api/login", headers, nil, c.configConnect.Version)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.DoLog(log.Log.Error, err.Error())
		}
	}()

	// parse the response
	switch {
	case resp == nil:
		return "", errNilResponse
	case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
		return "", c.api.ParseJSONError(resp)
	}

	// Extract token from response body
	token, err := extractString(resp)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (p *OIDCTokenProvider) GetToken(ctx context.Context) (*oauth2.Token, error) {
	// rp library expects issuer base URL; discovery uses /.well-known/openid-configuration
	rpClient, err := rp.NewRelyingPartyOIDC(
		ctx,
		p.cfg.Issuer,
		p.cfg.ClientID,
		p.cfg.ClientSecret,
		"",
		p.cfg.Scopes,
		rp.WithHTTPClient(p.httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC relying party client: %w", err)
	}

	token, err := rp.ClientCredentials(ctx, rpClient, nil)
	if err != nil {
		return nil, fmt.Errorf("client credentials: %w", err)
	}

	return token, nil
}

type TokenProvider interface {
	GetToken(ctx context.Context) (*oauth2.Token, error)
}

func WithHTTPClient(c *http.Client) OIDCOption {
	return func(o *oidcOpts) { o.httpClient = c }
}

func WithTimeout(d time.Duration) OIDCOption {
	return func(o *oidcOpts) { o.timeout = d }
}

func WithInsecureSkipVerify(skip bool) OIDCOption {
	return func(o *oidcOpts) { o.insecureSkipVerify = skip }
}

func NewTokenProvider(config *ConfigConnect, opts ...OIDCOption) (TokenProvider, error) {
	// Validate essentials
	if config == nil {
		return nil, fmt.Errorf("array connection data is nil")
	}
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, fmt.Errorf("issuer must not be empty")
	}
	if strings.TrimSpace(config.OidcClientID) == "" || strings.TrimSpace(config.OidcClientSecret) == "" {
		return nil, fmt.Errorf("oidc client credentials must not be empty (OidcClientID/OidcClientSecret)")
	}

	if config.Scopes == nil {
		config.Scopes = mergedScopes(config)
	}
	cfg := OIDCConfig{
		Issuer:       config.Issuer,
		ClientID:     config.OidcClientID,
		ClientSecret: config.OidcClientSecret,
		Scopes:       config.Scopes,
	}

	return NewOIDCTokenProvider(cfg, opts...)
}

func mergedScopes(config *ConfigConnect) []string {
	iss := strings.ToLower(strings.TrimSpace(config.Issuer))
	switch {
	case strings.Contains(iss, "login.microsoftonline.com") || strings.HasSuffix(iss, "/v2.0"):
		// Azure Scope: {"api://{resource-app-id}/.default"}
		// If you have a dedicated Resource App ID, use that instead of OidcClientID.
		return []string{fmt.Sprintf("api://%s/.default", config.OidcClientID)}

	case strings.Contains(iss, "/realms/"):
		// Keycloak Scope: {"openid"}
		return []string{"openid"}
	default:
		// Unknown provider: keep empty unless caller specifies explicitly via options/fields
		return nil
	}
}

type OIDCOption func(*oidcOpts)

type oidcOpts struct {
	httpClient         *http.Client
	timeout            time.Duration
	insecureSkipVerify bool
}

type OIDCConfig struct {
	// Base issuer URL (realm base for Keycloak, tenant base for Azure)
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type OIDCTokenProvider struct {
	cfg        OIDCConfig
	httpClient *http.Client
}

func NewOIDCTokenProvider(cfg OIDCConfig, opts ...OIDCOption) (*OIDCTokenProvider, error) {
	opt := &oidcOpts{}
	for _, f := range opts {
		f(opt)
	}
	httpClient, err := buildHTTPClient(opt)
	if err != nil {
		return nil, err
	}
	return &OIDCTokenProvider{cfg: cfg, httpClient: httpClient}, nil
}

func buildHTTPClient(o *oidcOpts) (*http.Client, error) {
	if o.httpClient != nil {
		return o.httpClient, nil
	}
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if o.insecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true
	}
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	client := &http.Client{Transport: transport}
	if o.timeout > 0 {
		client.Timeout = o.timeout
	}
	return client, nil
}

func (c *Client) oidcAuthenticate(configConnect *ConfigConnect) (string, error) {
	ctx := c.Context()

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	idpProvider, err := NewTokenProvider(
		configConnect,
		WithInsecureSkipVerify(configConnect.Insecure),
		WithTimeout(20*time.Second),
	)

	idpToken, err := idpProvider.GetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("Failed to get IDP token %s", err.Error())
	}

	form.Set("subject_token", idpToken.AccessToken)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://%s/rest/v1/token", configConnect.PfmpIP), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("ClientId", configConnect.CiamClientID)
	req.Header.Set("ClientSecret", configConnect.CiamClientSecret)

	client := &http.Client{
		Transport: &http.Transport{
			// #nosec G402
			TLSClientConfig: &tls.Config{InsecureSkipVerify: configConnect.Insecure}, // same as curl -k
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch {
	case resp == nil:
		return "", errNilResponse
	case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
		return "", c.api.ParseJSONError(resp)
	}
	return extractOauth2Token(resp), nil
}

func extractOauth2Token(response *http.Response) string {
	token := &oauth2.Token{}
	err := json.NewDecoder(response.Body).Decode(token)
	if err != nil {
		log.DoLog(log.Log.Error, err.Error())
	}
	return token.AccessToken
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (c *Client) xmlRequest(method, uri string, body, resp interface{}) (*http.Response, error) {
	response, err := c.api.DoXMLRequest(context.Background(), method, uri, c.configConnect.Version, body, resp)
	if err != nil {
		log.DoLog(log.Log.Error, err.Error())
	}
	return response, err
}

func (c *Client) getJSONWithRetry(
	method, uri string,
	body, resp interface{},
) error {
	return getJSONWithRetryFunc(c, method, uri, body, resp)
}

var getJSONWithRetryFunc = func(c *Client, method, uri string, body, resp interface{}) error {
	headers := make(map[string]string, 2)
	headers[api.HeaderKeyAccept] = accHeader
	headers[api.HeaderKeyContentType] = conHeader
	addMetaData(headers, body)

	ctx := c.Context()
	defer c.ResetContext()

	err := c.api.DoWithHeaders(
		ctx, method, uri, headers, body, resp, c.configConnect.Version)
	if err == nil {
		return nil
	}

	// check if we need to authenticate
	if e, ok := err.(*types.Error); ok {
		log.DoLog(log.Log.Debug, err.Error())
		if e.HTTPStatusCode == 401 {
			log.DoLog(log.Log.Info, "Need to re-auth")
			// Authenticate then try again
			if _, err := c.Authenticate(c.configConnect); err != nil {
				return fmt.Errorf("Error Authenticating: %s", err)
			}
			return c.api.DoWithHeaders(
				ctx, method, uri, headers, body, resp, c.configConnect.Version)
		}
	}
	log.DoLog(log.Log.Error, err.Error())

	return err
}

func extractString(resp *http.Response) (string, error) {
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errBodyRead
	}

	s := string(bs)

	// Remove any whitespace that might surround the JSON
	// JSON tokens are separated by whitespace, but there is no specification
	// determining what whitespace is to be expected so we have to prepare for the worst
	// see: https://github.com/golang/go/issues/7767#issuecomment-66093559
	s = strings.TrimSpace(s)

	s = strings.TrimLeft(s, `"`)
	s = strings.TrimRight(s, `"`)

	return s, nil
}

func (c *Client) getStringWithRetry(
	method, uri string,
	body interface{},
) (string, error) {
	headers := make(map[string]string, 2)
	headers[api.HeaderKeyAccept] = accHeader
	headers[api.HeaderKeyContentType] = conHeader
	addMetaData(headers, body)

	ctx := c.Context()
	defer c.ResetContext()

	checkResponse := func(resp *http.Response) (string, bool, error) {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.DoLog(log.Log.Error, err.Error())
			}
		}()

		// parse the response
		switch {
		case resp == nil:
			return "", false, errNilResponse
		case resp.StatusCode == 401:
			return "", true, c.api.ParseJSONError(resp)
		case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
			return "", false, c.api.ParseJSONError(resp)
		}

		s, err := extractString(resp)
		if err != nil {
			return "", false, err
		}

		return s, false, nil
	}

	resp, err := c.api.DoAndGetResponseBody(
		ctx, method, uri, headers, body, c.configConnect.Version)
	if err != nil {
		return "", err
	}
	s, retry, httpErr := checkResponse(resp)
	if httpErr != nil {
		if retry {
			log.DoLog(log.Log.Info, "need to re-auth")
			// Authenticate then try again
			if _, err = c.Authenticate(c.configConnect); err != nil {
				return "", fmt.Errorf("Error Authenticating: %s", err)
			}
			resp, err = c.api.DoAndGetResponseBody(
				ctx, method, uri, headers, body, c.configConnect.Version)
			if err != nil {
				return "", err
			}
			s, _, err = checkResponse(resp)
		} else {
			return "", httpErr
		}
	}

	return s, nil
}

// SetToken sets token
func (c *Client) SetToken(token string) {
	c.api.SetToken(token)
}

// GetToken returns token
func (c *Client) GetToken() string {
	return c.api.GetToken()
}

// GetConfigConnect returns Config of client
func (c *Client) GetConfigConnect() *ConfigConnect {
	return c.configConnect
}

func (c *Client) WithContext(ctx context.Context) *Client {
	c.ctx = ctx
	return c
}

func (c *Client) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}

	return context.Background()
}

func (c *Client) ResetContext() {
	c.ctx = nil
}

// NewClient returns a new client
func NewClient() (client *Client, err error) {
	return NewClientWithArgs(
		os.Getenv("GOSCALEIO_ENDPOINT"),
		os.Getenv("GOSCALEIO_VERSION"),
		math.MaxInt64,
		os.Getenv("GOSCALEIO_INSECURE") == "true",
		os.Getenv("GOSCALEIO_USECERTS") == "true")
}

// ClientConnectTimeout is used for unit testing to set the connection timeout much lower
var ClientConnectTimeout time.Duration

// NewClientWithArgs returns a new client
func NewClientWithArgs(
	endpoint string,
	version string,
	timeout int64,
	insecure,
	useCerts bool,
) (client *Client, err error) {
	if showHTTP {
		debug = true
	}
	if debug {
		log.SetLogLevel(slog.LevelDebug)
		log.DoLog(log.Log.Info, "Setting log level to debug in GoScaleIO")
	}

	fields := map[string]interface{}{
		"endpoint": endpoint,
		"insecure": insecure,
		"useCerts": useCerts,
		"version":  version,
		"debug":    debug,
		"showHTTP": showHTTP,
	}
	log.DoLog(log.Log.Debug, fmt.Sprintf("goscaleio client init, Fields: %+v", fields))

	if endpoint == "" {
		log.DoLog(log.Log.Error, fmt.Sprintf("endpoint is required, Fields: %+v", fields))
		return nil,
			withFields(fields, "endpoint is required")
	}

	opts := api.ClientOptions{
		Insecure: insecure,
		UseCerts: useCerts,
		ShowHTTP: showHTTP,
		Timeout:  time.Duration(timeout) * time.Second,
	}

	if ClientConnectTimeout != 0 {
		opts.Timeout = ClientConnectTimeout
	}

	ac, err := api.New(context.Background(), endpoint, opts, debug)
	if err != nil {
		log.DoLog(log.Log.Error, fmt.Sprintf("Unable to create HTTP client: %s", err.Error()))
		return nil, err
	}

	client = &Client{
		api: ac,
		configConnect: &ConfigConnect{
			Version: version,
		},
	}

	updateHeaders(version)

	return client, nil
}

// GetLink returns a link
func GetLink(links []*types.Link, rel string) (*types.Link, error) {
	for _, link := range links {
		if link.Rel == rel {
			return link, nil
		}
	}

	return nil, errNoLink
}

func withFields(fields map[string]interface{}, message string) error {
	return withFieldsE(fields, message, nil)
}

func withFieldsE(
	fields map[string]interface{}, message string, inner error,
) error {
	if fields == nil {
		fields = make(map[string]interface{})
	}

	if inner != nil {
		fields["inner"] = inner
	}

	x := 0
	l := len(fields)

	var b bytes.Buffer
	for k, v := range fields {
		if x < l-1 {
			b.WriteString(fmt.Sprintf("%s=%v,", k, v))
		} else {
			b.WriteString(fmt.Sprintf("%s=%v", k, v))
		}
		x = x + 1
	}

	return fmt.Errorf("%s %s", message, b.String())
}

// ExternalTimeRecorder is used to track time
var ExternalTimeRecorder func(string, time.Duration)

// TimeSpent is used to track time spent
func TimeSpent(functionName string, startTime time.Time) {
	if ExternalTimeRecorder != nil {
		endTime := time.Now()
		ExternalTimeRecorder(functionName, endTime.Sub(startTime))
	}
}

func addMetaData(headers map[string]string, body interface{}) {
	if headers == nil || body == nil {
		return
	}
	// If the body contains a MetaData method, extract the data
	// and add as HTTP headers.
	if vp, ok := interface{}(body).(interface {
		MetaData() http.Header
	}); ok {
		for k := range vp.MetaData() {
			headers[k] = vp.MetaData().Get(k)
		}
	}
}
