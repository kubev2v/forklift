package dynamic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// DataVolumeClient provides access to the provider's datavolume-source endpoint.
type DataVolumeClient struct {
	provider   *api.Provider
	httpClient *http.Client
}

// NewDataVolumeClient creates a new client for datavolume-source API calls.
func NewDataVolumeClient(provider *api.Provider) *DataVolumeClient {
	return &DataVolumeClient{
		provider: provider,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// DataVolumeSourceRequest represents the request body for datavolume-source endpoint.
type DataVolumeSourceRequest struct {
	TargetStorageClass string `json:"targetStorageClass"`
	TargetNamespace    string `json:"targetNamespace"`
}

// DataVolumeSourceResponse represents the response from datavolume-source endpoint.
type DataVolumeSourceResponse struct {
	Source   cdi.DataVolumeSource `json:"source"`
	Size     string               `json:"size"`
	Metadata map[string]string    `json:"metadata,omitempty"`
}

// V2VInputTypeResponse represents the response from v2v-input-type endpoint.
type V2VInputTypeResponse struct {
	InputType string `json:"inputType"` // "ova" or "libvirtxml"
}

// GetDataVolumeSource calls the provider's datavolume-source endpoint to get
// the CDI DataVolumeSource configuration for a specific disk.
func (c *DataVolumeClient) GetDataVolumeSource(vmID, diskID, namespace, storageClass string) (*DataVolumeSourceResponse, error) {
	// Get provider service URL from status
	if c.provider.Status.Service == nil {
		return nil, liberr.New("provider service not available in status")
	}

	serviceURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		c.provider.Status.Service.Name,
		c.provider.Status.Service.Namespace,
		c.provider.Status.Service.Port)

	// Build endpoint URL
	endpoint := fmt.Sprintf("%s/vms/%s/disks/%s/datavolume-source", serviceURL, vmID, diskID)

	// Build request body
	reqBody := DataVolumeSourceRequest{
		TargetStorageClass: storageClass,
		TargetNamespace:    namespace,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to marshal request")
	}

	// Make HTTP POST request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to call datavolume-source endpoint", "endpoint", endpoint)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, liberr.New(
			fmt.Sprintf("datavolume-source endpoint returned error: %d", resp.StatusCode),
			"response", string(body))
	}

	// Parse response
	var response DataVolumeSourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, liberr.Wrap(err, "failed to decode response")
	}

	return &response, nil
}

// GetV2VInputType calls the provider's v2v-input-type endpoint to determine
// which virt-v2v input mode to use for this VM.
func (c *DataVolumeClient) GetV2VInputType(vmID string) (*V2VInputTypeResponse, error) {
	// Get provider service URL from status
	if c.provider.Status.Service == nil {
		return nil, liberr.New("provider service not available in status")
	}

	serviceURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		c.provider.Status.Service.Name,
		c.provider.Status.Service.Namespace,
		c.provider.Status.Service.Port)

	// Build endpoint URL
	endpoint := fmt.Sprintf("%s/vms/%s/v2v-input-type", serviceURL, vmID)

	// Make HTTP GET request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to call v2v-input-type endpoint", "endpoint", endpoint)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, liberr.New(
			fmt.Sprintf("v2v-input-type endpoint returned error: %d", resp.StatusCode),
			"response", string(body))
	}

	// Parse response
	var response V2VInputTypeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, liberr.Wrap(err, "failed to decode response")
	}

	return &response, nil
}
