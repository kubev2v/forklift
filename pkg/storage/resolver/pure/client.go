package pure

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// RestClient provides authenticated REST API access to a Pure FlashArray.
// It is shared between the CSI import resolver and the xcopy volume populator.
type RestClient struct {
	hostname   string
	scheme     string // URL scheme; defaults to "https" when empty
	httpClient *http.Client
	apiToken   string
	authToken  string
	apiV1      string // Latest 1.x API version
	apiV2      string // Latest 2.x API version
}

// url returns a full URL for the given API path on this FlashArray.
func (c *RestClient) url(path string) string {
	scheme := c.scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + c.hostname + "/" + strings.TrimPrefix(path, "/")
}

// NewRestClient creates a new RestClient for the given FlashArray.
// hostname must be a bare host/IP (no scheme). If apiToken is non-empty it is
// used directly; otherwise username/password are used to obtain one.
func NewRestClient(hostname, username, password, apiToken string, skipSSL bool) (*RestClient, error) {
	c := &RestClient{
		hostname: hostname,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSL}, //nolint:gosec
			},
		},
	}

	if err := c.detectAPIVersions(); err != nil {
		return nil, fmt.Errorf("failed to detect Pure API versions at %s: %w", hostname, err)
	}

	if apiToken != "" {
		c.apiToken = apiToken
	} else {
		if err := c.getAPIToken(username, password); err != nil {
			return nil, fmt.Errorf("failed to obtain API token from Pure at %s: %w", hostname, err)
		}
	}

	if err := c.getAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to authenticate to Pure at %s: %w", hostname, err)
	}
	return c, nil
}

// FindVolumeByVVolID returns the volume name for a given VVol ID via the VASA tags API.
// vvolID may be in "vvol:<uuid>" or "rfc4122.<uuid>" format — both are handled.
// Note: resource_destroyed=False is intentionally omitted from the query because
// combining it with the filter= parameter causes Pure's API to return empty results.
func (c *RestClient) FindVolumeByVVolID(vvolID string) (string, error) {
	uuid := strings.TrimPrefix(vvolID, "vvol:")

	params := url.Values{}
	params.Set("namespaces", "vasa-integration.purestorage.com")
	params.Set("filter", fmt.Sprintf("key='PURE_VVOL_ID' AND value='%s'", uuid))
	rawURL := c.url(fmt.Sprintf("api/%s/volumes/tags", c.apiV2)) + "?" + params.Encode()

	body, err := c.doGet(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to query VVol tags from Pure array (VVolID: %s): %w", vvolID, err)
	}

	var result struct {
		Items []struct {
			Resource struct {
				Name string `json:"name"`
			} `json:"resource"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Pure tags response: %w", err)
	}
	if len(result.Items) == 0 {
		return "", fmt.Errorf("no Pure volume found with VVol ID: %s", vvolID)
	}
	return result.Items[0].Resource.Name, nil
}

// FindVolumeBySerial returns the volume name for a given serial number.
func (c *RestClient) FindVolumeBySerial(serial string) (string, error) {
	params := url.Values{}
	params.Set("filter", fmt.Sprintf("serial='%s'", strings.ToUpper(serial)))
	rawURL := c.url(fmt.Sprintf("api/%s/volumes", c.apiV2)) + "?" + params.Encode()

	body, err := c.doGet(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to query volumes from Pure array (serial: %s): %w", serial, err)
	}

	var result struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Pure volumes response: %w", err)
	}
	if len(result.Items) == 0 {
		return "", fmt.Errorf("no Pure volume found with serial: %s", serial)
	}
	return result.Items[0].Name, nil
}

// APIV2 returns the detected latest v2.x API version string.
func (c *RestClient) APIV2() string { return c.apiV2 }

// AuthToken returns the current x-auth-token for use in additional requests.
func (c *RestClient) AuthToken() string { return c.authToken }

// Hostname returns the FlashArray hostname.
func (c *RestClient) Hostname() string { return c.hostname }

// doGet performs a GET with auth-token and returns the body bytes.
// Retries once on 401 by refreshing the auth token.
func (c *RestClient) doGet(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		if err := c.getAuthToken(); err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}
		req, _ = http.NewRequest("GET", rawURL, nil)
		req.Header.Set("x-auth-token", c.authToken)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pure API returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *RestClient) detectAPIVersions() error {
	resp, err := c.httpClient.Get(c.url("api/api_version"))
	if err != nil {
		return fmt.Errorf("failed to get API versions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read API version response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API version request returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Version []string `json:"version"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse API version response: %w", err)
	}

	var v1s, v2s []string
	for _, v := range result.Version {
		switch {
		case strings.HasPrefix(v, "1."):
			v1s = append(v1s, v)
		case strings.HasPrefix(v, "2."):
			v2s = append(v2s, v)
		}
	}
	if len(v1s) == 0 {
		return fmt.Errorf("no API v1.x versions found")
	}
	if len(v2s) == 0 {
		return fmt.Errorf("no API v2.x versions found")
	}

	sort.Slice(v1s, func(i, j int) bool { return compareVersions(v1s[i], v1s[j]) > 0 })
	sort.Slice(v2s, func(i, j int) bool { return compareVersions(v2s[i], v2s[j]) > 0 })
	c.apiV1 = v1s[0]
	c.apiV2 = v2s[0]
	return nil
}

func (c *RestClient) getAPIToken(username, password string) error {
	payload := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	req, err := http.NewRequest("POST",
		c.url(fmt.Sprintf("api/%s/auth/apitoken", c.apiV1)),
		strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to build apitoken request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("apitoken request to Pure failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read apitoken response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apitoken request to Pure returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		APIToken string `json:"api_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse apitoken response: %w", err)
	}
	if result.APIToken == "" {
		return fmt.Errorf("empty api_token in Pure response")
	}
	c.apiToken = result.APIToken
	return nil
}

func (c *RestClient) getAuthToken() error {
	req, err := http.NewRequest("POST", c.url(fmt.Sprintf("api/%s/login", c.apiV2)), nil)
	if err != nil {
		return fmt.Errorf("failed to build login request: %w", err)
	}
	req.Header.Set("api-token", c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login to Pure failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login to Pure returned %d: %s", resp.StatusCode, string(body))
	}

	token := resp.Header.Get("x-auth-token")
	if token == "" {
		return fmt.Errorf("no x-auth-token in Pure login response")
	}
	c.authToken = token
	return nil
}

// compareVersions compares two dot-separated version strings numerically.
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		var aNum, bNum int
		_, _ = fmt.Sscanf(aParts[i], "%d", &aNum)
		_, _ = fmt.Sscanf(bParts[i], "%d", &bNum)
		if aNum != bNum {
			return aNum - bNum
		}
	}
	return len(aParts) - len(bParts)
}
