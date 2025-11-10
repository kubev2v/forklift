package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// CreateProviderInput represents the input for CreateProvider
type CreateProviderInput struct {
	ProviderName           string `json:"provider_name" jsonschema:"required"`
	ProviderType           string `json:"provider_type" jsonschema:"required"`
	Namespace              string `json:"namespace,omitempty"`
	Secret                 string `json:"secret,omitempty"`
	URL                    string `json:"url,omitempty"`
	Username               string `json:"username,omitempty"`
	Password               string `json:"password,omitempty"`
	Cacert                 string `json:"cacert,omitempty"`
	InsecureSkipTLS        *bool  `json:"insecure_skip_tls,omitempty"`
	Token                  string `json:"token,omitempty"`
	VDDKInitImage          string `json:"vddk_init_image,omitempty"`
	SDKEndpoint            string `json:"sdk_endpoint,omitempty"`
	UseVDDKAIOOptimization *bool  `json:"use_vddk_aio_optimization,omitempty"`
	VDDKBufSizeIn64K       int    `json:"vddk_buf_size_in_64k,omitempty"`
	VDDKBufCount           int    `json:"vddk_buf_count,omitempty"`
	ProviderDomainName     string `json:"provider_domain_name,omitempty"`
	ProviderProjectName    string `json:"provider_project_name,omitempty"`
	ProviderRegionName     string `json:"provider_region_name,omitempty"`
	DryRun                 bool   `json:"dry_run,omitempty"`
}

// GetCreateProviderTool returns the tool definition
func GetCreateProviderTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "CreateProvider",
		Description: `Create a new provider for connecting to source virtualization platforms.

    Providers connect MTV to source virtualization platforms (vSphere, oVirt, OpenStack, OpenShift, OVA).
    Each provider type requires different authentication and connection parameters.

    Dry Run Mode: Set dry_run=true to see the command without executing (useful for teaching users)

    Provider Types and Required Parameters:
    - vSphere: url, username/password OR token, optional: cacert, vddk_init_image, sdk_endpoint
    - oVirt: url, username, password, optional: cacert
    - OpenStack: url, username, password, provider_domain_name, provider_project_name, provider_region_name
    - OpenShift: url, optional: token, optional: cacert
    - OVA: url

    Security Notes:
    - Use cacert parameter with certificate content or prefix with @ to load from file
    - Set insecure_skip_tls=True to skip TLS verification (not recommended for production)
    - For existing secrets, use the secret parameter instead of credentials

    Certificate Loading Examples:
    - Direct content: cacert="-----BEGIN CERTIFICATE-----
..."
    - From file: cacert="@/path/to/ca-cert.pem"

    vSphere-Specific Options:
    - sdk_endpoint: Set to 'esxi' for direct ESXi connection, 'vcenter' for vCenter (default)
    - vddk_init_image: Custom VDDK container image for disk transfers
    - use_vddk_aio_optimization: Enable VDDK AIO optimization for better performance
    - vddk_buf_size_in_64k: VDDK buffer size in 64K units
    - vddk_buf_count: VDDK buffer count for parallel operations

    Args:
        provider_name: Name for the new provider (required)
        provider_type: Type of provider - 'vsphere', 'ovirt', 'openstack', 'openshift', or 'ova' (required)
        dry_run: If true, shows the kubectl-mtv command instead of executing it (educational mode) (optional, default: false)
        namespace: Kubernetes namespace to create the provider in (optional)
        secret: Name of existing secret containing provider credentials (optional, alternative to individual credentials)
        url: Provider URL/endpoint (required for most provider types)
        username: Provider credentials username (required unless using secret or token)
        password: Provider credentials password (required unless using secret or token)
        cacert: Provider CA certificate content or @filename to load from file (optional)
        insecure_skip_tls: Skip TLS verification when connecting to the provider (optional, default False)
        token: Provider authentication token (used for OpenShift provider) (optional)
        vddk_init_image: Virtual Disk Development Kit (VDDK) container init image path (vSphere only)
        sdk_endpoint: SDK endpoint type for vSphere provider - 'vcenter' or 'esxi' (optional)
        use_vddk_aio_optimization: Enable VDDK AIO optimization for vSphere provider (optional)
        vddk_buf_size_in_64k: VDDK buffer size in 64K units (vSphere only, optional)
        vddk_buf_count: VDDK buffer count (vSphere only, optional)
        provider_domain_name: OpenStack domain name (OpenStack only)
        provider_project_name: OpenStack project name (OpenStack only)
        provider_region_name: OpenStack region name (OpenStack only)

    Returns:
        Command output confirming provider creation

    Examples:
        # Create vSphere provider with credentials
        create_provider("my-vsphere", "vsphere", url="https://vcenter.example.com",
                       username="admin", password="password123")

        # Create OpenStack provider
        create_provider("my-openstack", "openstack", url="https://keystone.example.com:5000/v3",
                       username="admin", password="password123", provider_domain_name="Default",
                       provider_project_name="admin", provider_region_name="RegionOne")

        # Create OpenShift provider with token
        create_provider("my-openshift", "openshift", url="https://api.ocp.example.com:6443",
                       token="sha256~abcdef...")`,
	}
}

func HandleCreateProvider(ctx context.Context, req *mcp.CallToolRequest, input CreateProviderInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"provider_name": input.ProviderName,
		"provider_type": input.ProviderType,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"create", "provider", "--type", input.ProviderType, input.ProviderName}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	// Add authentication parameters
	if input.Secret != "" {
		args = append(args, "--secret", input.Secret)
	}
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
	mtvmcp.AddBooleanFlag(&args, "provider-insecure-skip-tls", input.InsecureSkipTLS)
	if input.Token != "" {
		args = append(args, "--token", input.Token)
	}

	// vSphere-specific parameters
	if input.VDDKInitImage != "" {
		args = append(args, "--vddk-init-image", input.VDDKInitImage)
	}
	if input.SDKEndpoint != "" {
		args = append(args, "--sdk-endpoint", input.SDKEndpoint)
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

	result, err := mtvmcp.RunKubectlMTVCommand(ctx, args)
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
