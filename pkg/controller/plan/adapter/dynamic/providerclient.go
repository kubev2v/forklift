package dynamic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	dynamicregistry "github.com/kubev2v/forklift/pkg/controller/provider/web/dynamic"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

var clientLog = logging.WithName("dynamic|provider-client")

// ProviderClient calls the dynamic provider's build-spec API
type ProviderClient struct {
	*plancontext.Context
	httpClient *http.Client
}

// NewProviderClient creates a new client for calling the dynamic provider's API
func NewProviderClient(ctx *plancontext.Context) *ProviderClient {
	return &ProviderClient{
		Context: ctx,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// BuildVMSpec calls the dynamic provider's build-spec endpoint to build a VirtualMachine spec
func (c *ProviderClient) BuildVMSpec(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim, useInstanceType bool) (vm *kubevirtv1.VirtualMachine, err error) {
	providerType := string(c.Source.Provider.Type())

	// Get provider config from registry
	config, isDynamic := dynamicregistry.Registry.Get(providerType)
	if !isDynamic {
		err = liberr.New("Provider is not dynamic", "type", providerType)
		return
	}

	// Build request - provider server can query everything else from the cluster
	req := BuildSpecRequest{
		Plan: c.buildPlanInfo(vmRef),
	}

	// Call provider API
	url := fmt.Sprintf("%s/vms/%s/build-spec", config.ServiceURL, vmRef.ID)

	clientLog.Info("Calling provider build-spec API",
		"provider", c.Source.Provider.Name,
		"vmID", vmRef.ID,
		"url", url)

	resp, err := c.callBuildSpecAPI(url, req)
	if err != nil {
		return
	}

	vm = resp.VirtualMachine

	clientLog.Info("Successfully built VM spec from provider",
		"provider", c.Source.Provider.Name,
		"vmID", vmRef.ID,
		"vmName", vm.Name,
		"detectedOS", resp.Metadata.DetectedOS)

	return
}

// callBuildSpecAPI makes the HTTP POST request to the provider's build-spec endpoint
func (c *ProviderClient) callBuildSpecAPI(url string, req BuildSpecRequest) (*BuildSpecResponse, error) {
	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to marshal request")
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create HTTP request")
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to call provider API", "url", url)
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, liberr.New(
			"provider API returned error",
			"status", httpResp.StatusCode,
			"body", string(body))
	}

	// Parse response
	var resp BuildSpecResponse
	if err = json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, liberr.Wrap(err, "failed to decode response")
	}

	return &resp, nil
}

// buildPlanInfo builds plan reference
func (c *ProviderClient) buildPlanInfo(vmRef ref.Ref) PlanInfo {
	return PlanInfo{
		Name:      c.Plan.Name,
		Namespace: c.Plan.Namespace,
	}
}

// BuildSpecRequest mirrors the request structure in the provider server
type BuildSpecRequest struct {
	Plan PlanInfo `json:"plan"`
}

// PlanInfo contains the plan reference
type PlanInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// BuildSpecResponse mirrors the response structure from the provider server
type BuildSpecResponse struct {
	VirtualMachine *kubevirtv1.VirtualMachine `json:"virtualMachine"`
	Metadata       VMMetadata                 `json:"metadata"`
}

// VMMetadata contains additional VM metadata detected during building
type VMMetadata struct {
	DetectedOS         string `json:"detectedOS"`
	Firmware           string `json:"firmware"`
	SecureBoot         bool   `json:"secureBoot"`
	GuestToolsDetected bool   `json:"guestToolsDetected"`
}
