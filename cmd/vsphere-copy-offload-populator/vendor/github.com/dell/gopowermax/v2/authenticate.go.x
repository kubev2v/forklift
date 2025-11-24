package pmax

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/dell/gopowermax/v2/api"
	types "github.com/dell/gopowermax/v2/types/v100"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	configConnect *ConfigConnect
	api           api.Client
}

type ConfigConnect struct {
	Endpoint string
	Version  string
	Username string
	Password string
}

var (
	errNilReponse = errors.New("nil response from API")
	errBodyRead   = errors.New("error reading body")
	errNoLink     = errors.New("Error: problem finding link")
	debug, _      = strconv.ParseBool(os.Getenv("CSI_POWERMAX_DEBUG"))
	showHTTP, _   = strconv.ParseBool(os.Getenv("CSI_POWERMAX_SHOWHTTP"))
	accHeader     string
	conHeader     string
)

// Authenticate and get API version
func (c *Client) Authenticate(configConnect *ConfigConnect) error {
	c.configConnect = configConnect
	c.api.SetToken("")
	basicAuthString := basicAuth(configConnect.Username, configConnect.Password)

	headers := make(map[string]string, 1)
	headers["Authorization"] = "Basic " + basicAuthString

	resp, err := c.api.DoAndGetResponseBody(
		context.Background(), http.MethodGet, "univmax/restapi/system/version", headers, nil)
	if err != nil {
		doLog(log.WithError(err).Error, "")
		return err
	}
	defer resp.Body.Close()

	// parse the response
	switch {
	case resp == nil:
		return errNilReponse
	case !(resp.StatusCode >= 200 && resp.StatusCode <= 299):
		return c.api.ParseJSONError(resp)
	}
        c.api.SetToken(basicAuthString)
        version := &types.Version{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(version)
	if err != nil {
		return nil
        }
        log.Printf("API version: %s", version.Version)

	return nil
}

// Generate the base 64 Authorization string from username / password
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func doLog(
	l func(args ...interface{}),
	msg string) {

	if debug {
		l(msg)
	}
}

func NewClient() (client *Client, err error) {
	return NewClientWithArgs(
		os.Getenv("CSI_POWERMAX_ENDPOINT"),
		os.Getenv("CSI_POWERMAX_VERSION"),
		os.Getenv("CSI_POWERMAX_INSECURE") == "true",
		os.Getenv("CSI_POWERMAX_USECERTS") == "true")
}

func NewClientWithArgs(
	endpoint string,
	version string,
	insecure,
	useCerts bool) (client *Client, err error) {

	if showHTTP {
		debug = true
	}

	fields := map[string]interface{}{
		"endpoint": endpoint,
		"insecure": insecure,
		"useCerts": useCerts,
		"version":  version,
		"debug":    debug,
		"showHTTP": showHTTP,
	}

	doLog(log.WithFields(fields).Debug, "goscaleio client init")

	if endpoint == "" {
		doLog(log.WithFields(fields).Error, "endpoint is required")
		return nil, fmt.Errorf("Endpoint must be supplied, e.g. https://1.2.3.4:8443")
	}

	opts := api.ClientOptions{
		Insecure: insecure,
		UseCerts: useCerts,
		ShowHTTP: showHTTP,
	}

	ac, err := api.New(context.Background(), endpoint, opts, debug)
	if err != nil {
		doLog(log.WithError(err).Error, "Unable to create HTTP client")
		return nil, err
	}

	client = &Client{
		api: ac,
		configConnect: &ConfigConnect{
			Version: version,
		},
	}

	accHeader = api.HeaderValContentTypeJSON
	if version != "" {
		accHeader = accHeader + ";version=" + version
	}
	conHeader = accHeader

	return client, nil
}

func (c *Client) getDefaultHeaders() *map[string]string {
        headers := make(map[string]string)
        headers["Accept"] = accHeader
        headers["Content-Type"] = conHeader
	basicAuthString := basicAuth(configConnect.Username, configConnect.Password)
	headers["Authorization"] = "Basic " + basicAuthString
}
