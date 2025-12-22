package pure

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// RestClient provides REST API access to Pure FlashArray
type RestClient struct {
	hostname   string
	httpClient *http.Client
	apiToken   string
	authToken  string
	apiV1      string // Latest 1.x API version
	apiV2      string // Latest 2.x API version
}

// APIVersionResponse represents the response from /api/api_version
type APIVersionResponse struct {
	Version []string `json:"version"`
}

// APITokenRequest represents the request for getting an API token
type APITokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// APITokenResponse represents the response containing the API token
type APITokenResponse struct {
	APIToken string `json:"api_token"`
}

// VolumeTagItem represents a volume tag item from the tags API
type VolumeTagItem struct {
	Namespace string `json:"namespace"`
	Value     string `json:"value"`
	Resource  struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"resource"`
	Key      string `json:"key"`
	Copyable bool   `json:"copyable"`
}

// VolumeTagsResponse represents the response from the volume tags API
type VolumeTagsResponse struct {
	Items              []VolumeTagItem `json:"items"`
	ContinuationToken  *string         `json:"continuation_token"`
	MoreItemsRemaining bool            `json:"more_items_remaining"`
	TotalItemCount     *int            `json:"total_item_count"`
}

// CopyVolumeRequest represents the request for copying a volume
type CopyVolumeRequest struct {
	Source struct {
		Name string `json:"name"`
	} `json:"source"`
	Names string `json:"names"` // Changed from []string to string
}

// Host represents a Pure FlashArray host
type Host struct {
	Name string   `json:"name"`
	Iqn  []string `json:"iqns"`
	Wwn  []string `json:"wwns"`
}

// Volume represents a Pure FlashArray volume
type Volume struct {
	Name   string `json:"name"`
	Serial string `json:"serial"`
	Size   uint64 `json:"size"`
}

// HostsResponse represents the response from hosts API
type HostsResponse struct {
	Items []Host `json:"items"`
}

// VolumesResponse represents the response from volumes API
type VolumesResponse struct {
	Items []Volume `json:"items"`
}

// HostConnectionRequest represents a host connection request
type HostConnectionRequest struct {
	HostNames   string `json:"host_names"`
	VolumeNames string `json:"volume_names"`
}

// NewRestClient creates a new REST client for Pure FlashArray
func NewRestClient(hostname, username, password string, skipSSLVerify bool) (*RestClient, error) {
	client := &RestClient{
		hostname: hostname,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: skipSSLVerify,
				},
			},
		},
	}

	// Step 1: Detect available API versions
	if err := client.detectAPIVersions(); err != nil {
		return nil, fmt.Errorf("failed to detect API versions: %w", err)
	}

	// Step 2: Get API token using latest 1.x API (only 1.x supports this)
	if err := client.getAPIToken(username, password); err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	// Step 3: Get auth token using latest 2.x API
	if err := client.getAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	klog.Infof("Pure REST Client: Successfully initialized with API v%s (token)/v%s (operations)", client.apiV1, client.apiV2)
	return client, nil
}

// detectAPIVersions detects available API versions and selects the latest 1.x and 2.x versions
func (c *RestClient) detectAPIVersions() error {
	url := fmt.Sprintf("https://%s/api/api_version", c.hostname)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get API versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API version request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read API version response: %w", err)
	}

	var apiResponse APIVersionResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return fmt.Errorf("failed to parse API version response: %w", err)
	}

	// Find latest 1.x and 2.x versions
	var v1Versions, v2Versions []string

	for _, version := range apiResponse.Version {
		if strings.HasPrefix(version, "1.") {
			v1Versions = append(v1Versions, version)
		} else if strings.HasPrefix(version, "2.") {
			v2Versions = append(v2Versions, version)
		}
	}

	if len(v1Versions) == 0 {
		return fmt.Errorf("no API v1.x versions found")
	}
	if len(v2Versions) == 0 {
		return fmt.Errorf("no API v2.x versions found")
	}

	// Sort to get the latest versions
	sort.Slice(v1Versions, func(i, j int) bool {
		return compareVersions(v1Versions[i], v1Versions[j]) > 0
	})
	sort.Slice(v2Versions, func(i, j int) bool {
		return compareVersions(v2Versions[i], v2Versions[j]) > 0
	})

	c.apiV1 = v1Versions[0]
	c.apiV2 = v2Versions[0]

	klog.Infof("Pure REST Client: Using API versions v%s (token)/v%s (operations)", c.apiV1, c.apiV2)
	return nil
}

// getAPIToken gets an API token using username/password via latest 1.x API
func (c *RestClient) getAPIToken(username, password string) error {
	url := fmt.Sprintf("https://%s/api/%s/auth/apitoken", c.hostname, c.apiV1)

	requestBody := APITokenRequest{
		Username: username,
		Password: password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal API token request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create API token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send API token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read API token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse APITokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return fmt.Errorf("failed to parse API token response: %w", err)
	}

	c.apiToken = tokenResponse.APIToken
	return nil
}

// getAuthToken gets an authentication token using API token via latest 2.x API
func (c *RestClient) getAuthToken() error {
	url := fmt.Sprintf("https://%s/api/%s/login", c.hostname, c.apiV2)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create auth token request: %w", err)
	}

	req.Header.Set("api-token", c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read auth token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	authToken := resp.Header.Get("x-auth-token")
	if authToken == "" {
		return fmt.Errorf("no x-auth-token header in response")
	}

	c.authToken = authToken
	return nil
}

// FindVolumeByVVolID finds a volume using its VVol ID via the tags API
func (c *RestClient) FindVolumeByVVolID(vvolID string) (string, error) {
	filter := fmt.Sprintf("key='PURE_VVOL_ID' AND value='%s'", vvolID)

	baseURL := fmt.Sprintf("https://%s/api/%s/volumes/tags", c.hostname, c.apiV2)

	params := url.Values{}
	params.Set("resource_destroyed", "False")
	params.Set("namespaces", "vasa-integration.purestorage.com")
	params.Set("filter", filter)

	finalURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create volume search request: %w", err)
	}

	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send volume search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read volume search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("volume search request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResponse VolumeTagsResponse
	if err := json.Unmarshal(body, &tagsResponse); err != nil {
		return "", fmt.Errorf("failed to parse volume search response: %w", err)
	}

	if len(tagsResponse.Items) == 0 {
		return "", fmt.Errorf("no volume found with VVol ID: %s", vvolID)
	}

	volumeName := tagsResponse.Items[0].Resource.Name
	klog.Infof("Pure REST Client: Found volume %s for VVol ID %s", volumeName, vvolID)
	return volumeName, nil
}

// CopyVolume copies a volume from source to target
func (c *RestClient) CopyVolume(sourceVolumeName, targetVolumeName string) error {
	url := fmt.Sprintf("https://%s/api/%s/volumes?overwrite=true", c.hostname, c.apiV2)

	requestBody := CopyVolumeRequest{
		Names: targetVolumeName,
	}
	requestBody.Source.Name = sourceVolumeName

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal copy volume request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create copy volume request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send copy volume request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read copy volume response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("copy volume request failed with status %d: %s", resp.StatusCode, string(body))
	}

	klog.Infof("Pure REST Client: Successfully copied volume from %s to %s", sourceVolumeName, targetVolumeName)
	return nil
}

// ListHosts lists all local hosts on the Pure FlashArray
func (c *RestClient) ListHosts() ([]Host, error) {
	// Filter for local hosts only to avoid selecting remote hosts in active cluster setups
	url := fmt.Sprintf("https://%s/api/%s/hosts?filter=is_local", c.hostname, c.apiV2)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list hosts request: %w", err)
	}

	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send list hosts request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list hosts response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list hosts request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var hostsResponse HostsResponse
	if err := json.Unmarshal(body, &hostsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse list hosts response: %w", err)
	}

	return hostsResponse.Items, nil
}

// ConnectHost connects a volume to a host
func (c *RestClient) ConnectHost(hostName, volumeName string) error {
	url := fmt.Sprintf("https://%s/api/%s/connections", c.hostname, c.apiV2)

	requestBody := HostConnectionRequest{
		HostNames:   hostName,
		VolumeNames: volumeName,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal connect host request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create connect host request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send connect host request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read connect host response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("connect host request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DisconnectHost disconnects a volume from a host
func (c *RestClient) DisconnectHost(hostName, volumeName string) error {
	url := fmt.Sprintf("https://%s/api/%s/connections?host_names=%s&volume_names=%s", c.hostname, c.apiV2, hostName, volumeName)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create disconnect host request: %w", err)
	}

	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send disconnect host request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read disconnect host response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("disconnect host request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetVolume gets information about a specific volume
func (c *RestClient) GetVolume(volumeName string) (*Volume, error) {
	baseURL := fmt.Sprintf("https://%s/api/%s/volumes", c.hostname, c.apiV2)

	params := url.Values{}
	params.Set("names", volumeName)

	finalURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get volume request: %w", err)
	}

	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send get volume request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read get volume response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get volume request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var volumesResponse VolumesResponse
	if err := json.Unmarshal(body, &volumesResponse); err != nil {
		return nil, fmt.Errorf("failed to parse get volume response: %w", err)
	}

	if len(volumesResponse.Items) == 0 {
		return nil, fmt.Errorf("volume not found: %s", volumeName)
	}

	return &volumesResponse.Items[0], nil
}

// ListVolumes lists all volumes on the Pure FlashArray
func (c *RestClient) ListVolumes() ([]Volume, error) {
	url := fmt.Sprintf("https://%s/api/%s/volumes", c.hostname, c.apiV2)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list volumes request: %w", err)
	}

	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send list volumes request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list volumes response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list volumes request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var volumesResponse VolumesResponse
	if err := json.Unmarshal(body, &volumesResponse); err != nil {
		return nil, fmt.Errorf("failed to parse list volumes response: %w", err)
	}

	return volumesResponse.Items, nil
}

// FindVolumeBySerial finds a volume by its serial number
func (c *RestClient) FindVolumeBySerial(serial string) (*Volume, error) {
	// Pure FlashArray API allows filtering by serial
	baseURL := fmt.Sprintf("https://%s/api/%s/volumes", c.hostname, c.apiV2)

	// Normalize serial to uppercase for comparison
	serial = strings.ToUpper(serial)

	params := url.Values{}
	params.Set("filter", fmt.Sprintf("serial='%s'", serial))

	finalURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create find volume request: %w", err)
	}

	req.Header.Set("x-auth-token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send find volume request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read find volume response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("find volume request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var volumesResponse VolumesResponse
	if err := json.Unmarshal(body, &volumesResponse); err != nil {
		return nil, fmt.Errorf("failed to parse find volume response: %w", err)
	}

	if len(volumesResponse.Items) == 0 {
		return nil, fmt.Errorf("volume not found with serial: %s", serial)
	}

	klog.Infof("Pure REST Client: Found volume %s for serial %s", volumesResponse.Items[0].Name, serial)
	return &volumesResponse.Items[0], nil
}

// compareVersions compares two version strings (e.g., "1.19" vs "1.2")
// Returns > 0 if v1 > v2, 0 if equal, < 0 if v1 < v2
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			p1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			p2, _ = strconv.Atoi(parts2[i])
		}

		if p1 != p2 {
			return p1 - p2
		}
	}

	return 0
}
