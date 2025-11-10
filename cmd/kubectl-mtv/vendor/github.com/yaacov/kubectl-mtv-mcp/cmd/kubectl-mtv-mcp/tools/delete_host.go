package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// DeleteHostInput represents the input for DeleteHost
type DeleteHostInput struct {
	HostName  string `json:"host_name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	AllHosts  bool   `json:"all_hosts,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

// GetDeleteHostTool returns the tool definition
func GetDeleteHostTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "DeleteHost",
		Description: `Delete one or more migration hosts.

    WARNING: This will remove migration hosts.

    Args:
        host_name: Name of the host to delete (required unless all_hosts=True)
        dry_run: If true, shows the kubectl-mtv command instead of executing it (educational mode) (optional, default: false)
        namespace: Kubernetes namespace containing the host (optional)
        all_hosts: Delete all hosts in the namespace (optional)

    Returns:
        Command output confirming host deletion

    Examples:
        # Delete specific host
        DeleteHost(host_name="esxi-host-01")

        # Delete all hosts in namespace
        DeleteHost(all_hosts=true, namespace="demo")`,
	}
}

func HandleDeleteHost(ctx context.Context, req *mcp.CallToolRequest, input DeleteHostInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	args := []string{"delete", "host"}

	if input.AllHosts {
		args = append(args, "--all")
	} else {
		if input.HostName == "" {
			return nil, "", fmt.Errorf("host_name is required when all_hosts=false")
		}
		args = append(args, input.HostName)
	}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
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
