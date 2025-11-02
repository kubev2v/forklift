package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// PatchProviderInput represents the input for PatchProvider
type PatchProviderInput struct {
	ProviderName           string `json:"provider_name" jsonschema:"required"`
	Namespace              string `json:"namespace,omitempty"`
	URL                    string `json:"url,omitempty"`
	Username               string `json:"username,omitempty"`
	Password               string `json:"password,omitempty"`
	Cacert                 string `json:"cacert,omitempty"`
	InsecureSkipTLS        *bool  `json:"insecure_skip_tls,omitempty"`
	Token                  string `json:"token,omitempty"`
	VDDKInitImage          string `json:"vddk_init_image,omitempty"`
	UseVDDKAIOOptimization *bool  `json:"use_vddk_aio_optimization,omitempty"`
	VDDKBufSizeIn64K       int    `json:"vddk_buf_size_in_64k,omitempty"`
	VDDKBufCount           int    `json:"vddk_buf_count,omitempty"`
	ProviderDomainName     string `json:"provider_domain_name,omitempty"`
	ProviderProjectName    string `json:"provider_project_name,omitempty"`
	ProviderRegionName     string `json:"provider_region_name,omitempty"`
}

// GetPatchProviderTool returns the tool definition
func GetPatchProviderTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "PatchProvider",
		Description: `Patch/modify an existing provider by updating URL, credentials, or VDDK settings.

    This allows updating provider configuration without recreating it. Provider type and
    SDK endpoint cannot be changed through patching.

    Editable Provider Settings:
    - Authentication: URL, username, password, token, CA certificate
    - Security: TLS verification settings
    - vSphere VDDK: Init image, AIO optimization, buffer settings
    - OpenStack: Domain, project, and region names

    Certificate Loading:
    - Direct content: Pass certificate content as string
    - File loading: Use @filename syntax to load certificate from file

    Boolean Parameters:
    - None (default): Don't change the current value
    - True: Enable the setting
    - False: Disable the setting

    Args:
        provider_name: Name of the provider to patch (required)
        namespace: Kubernetes namespace containing the provider (optional)
        url: Provider URL/endpoint (optional)
        username: Provider credentials username (optional)
        password: Provider credentials password (optional)
        cacert: Provider CA certificate content or @filename (optional)
        insecure_skip_tls: Skip TLS verification when connecting (optional)
        token: Provider authentication token (for OpenShift) (optional)
        vddk_init_image: VDDK container init image path (vSphere only) (optional)
        use_vddk_aio_optimization: Enable VDDK AIO optimization (vSphere only) (optional)
        vddk_buf_size_in_64k: VDDK buffer size in 64K units (vSphere only) (optional)
        vddk_buf_count: VDDK buffer count (vSphere only) (optional)
        provider_domain_name: OpenStack domain name (OpenStack only) (optional)
        provider_project_name: OpenStack project name (OpenStack only) (optional)
        provider_region_name: OpenStack region name (OpenStack only) (optional)

    Returns:
        Command output confirming provider patch

    Examples:
        # Update vSphere provider credentials and VDDK settings
        patch_provider(provider_name="my-vsphere", url="https://new-vcenter.example.com",
                      username="newuser", vddk_init_image="my-registry/vddk:latest")

        # Update OpenStack provider region
        patch_provider(provider_name="my-openstack", provider_region_name="RegionTwo")

        # Enable VDDK optimization and increase buffer settings
        patch_provider(provider_name="my-vsphere", use_vddk_aio_optimization=true,
                      vddk_buf_count=32, vddk_buf_size_in_64k=128)`,
	}
}

func HandlePatchProvider(ctx context.Context, req *mcp.CallToolRequest, input PatchProviderInput) (*mcp.CallToolResult, any, error) {
	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"provider_name": input.ProviderName,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"patch", "provider", input.ProviderName}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	// Add authentication parameters
	if input.URL != "" {
		args = append(args, "--url", input.URL)
	}
	if input.Username != "" {
		args = append(args, "--username", input.Username)
	}
	if input.Password != "" {
		args = append(args, "--password", input.Password)
	}
	if input.Cacert != "" {
		args = append(args, "--cacert", input.Cacert)
	}
	if input.Token != "" {
		args = append(args, "--token", input.Token)
	}

	// Add security parameters
	mtvmcp.AddBooleanFlag(&args, "provider-insecure-skip-tls", input.InsecureSkipTLS)

	// vSphere-specific parameters
	if input.VDDKInitImage != "" {
		args = append(args, "--vddk-init-image", input.VDDKInitImage)
	}
	mtvmcp.AddBooleanFlag(&args, "use-vddk-aio-optimization", input.UseVDDKAIOOptimization)
	if input.VDDKBufSizeIn64K > 0 {
		args = append(args, "--vddk-buf-size-in-64k", fmt.Sprintf("%d", input.VDDKBufSizeIn64K))
	}
	if input.VDDKBufCount > 0 {
		args = append(args, "--vddk-buf-count", fmt.Sprintf("%d", input.VDDKBufCount))
	}

	// OpenStack-specific parameters
	if input.ProviderDomainName != "" {
		args = append(args, "--provider-domain-name", input.ProviderDomainName)
	}
	if input.ProviderProjectName != "" {
		args = append(args, "--provider-project-name", input.ProviderProjectName)
	}
	if input.ProviderRegionName != "" {
		args = append(args, "--provider-region-name", input.ProviderRegionName)
	}

	result, err := mtvmcp.RunKubectlMTVCommand(args)
	if err != nil {
		return nil, "", err
	}

	// Unmarshal the full CommandResponse to provide complete diagnostic information
	data, err := mtvmcp.UnmarshalJSONResponse(result)
	if err != nil {
		return nil, "", err
	}
	return nil, data, nil
}
