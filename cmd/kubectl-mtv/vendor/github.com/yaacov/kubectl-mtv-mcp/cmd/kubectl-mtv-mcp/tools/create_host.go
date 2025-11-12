package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// CreateHostInput represents the input for CreateHost
type CreateHostInput struct {
	HostName            string `json:"host_name" jsonschema:"required"`
	Provider            string `json:"provider" jsonschema:"required"`
	Namespace           string `json:"namespace,omitempty"`
	Username            string `json:"username,omitempty"`
	Password            string `json:"password,omitempty"`
	ExistingSecret      string `json:"existing_secret,omitempty"`
	IPAddress           string `json:"ip_address,omitempty"`
	NetworkAdapter      string `json:"network_adapter,omitempty"`
	HostInsecureSkipTLS *bool  `json:"host_insecure_skip_tls,omitempty"`
	Cacert              string `json:"cacert,omitempty"`
	InventoryURL        string `json:"inventory_url,omitempty"`
	DryRun              bool   `json:"dry_run,omitempty"`
}

// GetCreateHostTool returns the tool definition
func GetCreateHostTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "CreateHost",
		Description: `Create migration hosts for vSphere providers to enable direct data transfer.

    Migration hosts enable direct data transfer from ESXi hosts, bypassing vCenter for improved
    performance. They allow Forklift to utilize ESXi host interfaces directly for network transfer
    to OpenShift, provided network connectivity exists between OpenShift worker nodes and ESXi hosts.

    Host creation is only supported for vSphere providers and requires the host to exist in the
    provider's inventory. ESXi endpoint providers can automatically use provider credentials.

    IP Address Resolution:
    - ip_address: Use specific IP address for direct connection
    - network_adapter: Use IP from named network adapter in inventory (e.g., "Management Network")

    Authentication Options:
    - existing_secret: Use existing Kubernetes secret with credentials
    - username/password: Create new credentials (will create secret automatically)
    - ESXi providers: Can automatically inherit provider credentials

    Args:
        host_name: Name of the host in provider inventory (required)
        provider: Name of vSphere provider (required)
        dry_run: If true, shows the kubectl-mtv command instead of executing it (educational mode) (optional, default: false)
        namespace: Kubernetes namespace to create the host in (optional)
        username: Username for host authentication (required unless using existing_secret or ESXi provider)
        password: Password for host authentication (required unless using existing_secret or ESXi provider)
        existing_secret: Name of existing secret for host authentication (optional)
        ip_address: IP address for disk transfer (mutually exclusive with network_adapter)
        network_adapter: Network adapter name to get IP from inventory (mutually exclusive with ip_address)
        host_insecure_skip_tls: Skip TLS verification for host connection (optional, default False)
        cacert: CA certificate content or @filename to load from file (optional)
        inventory_url: Base URL for inventory service (optional, auto-discovered if not provided)

    Returns:
        Command output confirming host creation

    Examples:
        # Create host with direct IP using existing secret
        create_host("esxi-host-01", "my-vsphere-provider",
                   existing_secret="esxi-credentials", ip_address="192.168.1.10")

        # Create host with network adapter lookup
        create_host("esxi-host-01", "my-vsphere-provider",
                   username="root", password="password123",
                   network_adapter="Management Network")

        # Create host for ESXi endpoint provider (inherits credentials)
        create_host("esxi-host-01", "my-esxi-provider", ip_address="192.168.1.10")`,
	}
}

func HandleCreateHost(ctx context.Context, req *mcp.CallToolRequest, input CreateHostInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"host_name": input.HostName,
		"provider":  input.Provider,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"create", "host", input.HostName}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	if input.Provider != "" {
		args = append(args, "--provider", input.Provider)
	}
	if input.Username != "" {
		args = append(args, "--username", input.Username)
	}
	if input.Password != "" {
		args = append(args, "--password", input.Password)
	}
	if input.ExistingSecret != "" {
		args = append(args, "--existing-secret", input.ExistingSecret)
	}
	if input.IPAddress != "" {
		args = append(args, "--ip-address", input.IPAddress)
	}
	if input.NetworkAdapter != "" {
		args = append(args, "--network-adapter", input.NetworkAdapter)
	}
	mtvmcp.AddBooleanFlag(&args, "host-insecure-skip-tls", input.HostInsecureSkipTLS)
	if input.Cacert != "" {
		args = append(args, "--cacert", input.Cacert)
	}
	if input.InventoryURL != "" {
		args = append(args, "--inventory-url", input.InventoryURL)
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
