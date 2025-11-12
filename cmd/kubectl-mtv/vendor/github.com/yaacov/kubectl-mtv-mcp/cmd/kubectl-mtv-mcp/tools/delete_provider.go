package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// DeleteProviderInput represents the input for DeleteProvider
type DeleteProviderInput struct {
	ProviderName string `json:"provider_name,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	AllProviders bool   `json:"all_providers,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
}

// GetDeleteProviderTool returns the tool definition
func GetDeleteProviderTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "DeleteProvider",
		Description: `Delete one or more providers.

    WARNING: This will remove providers and may affect associated plans and mappings.

    Args:
        provider_name: Name of the provider to delete (required unless all_providers=True)
        dry_run: If true, shows the kubectl-mtv command instead of executing it (educational mode) (optional, default: false)
        namespace: Kubernetes namespace containing the provider (optional)
        all_providers: Delete all providers in the namespace (optional)

    Returns:
        Command output confirming provider deletion

    Examples:
        # Delete specific provider
        DeleteProvider(provider_name="my-provider")

        # Delete all providers in namespace
        DeleteProvider(all_providers=true, namespace="demo")`,
	}
}

func HandleDeleteProvider(ctx context.Context, req *mcp.CallToolRequest, input DeleteProviderInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	args := []string{"delete", "provider"}

	if input.AllProviders {
		args = append(args, "--all")
	} else {
		if input.ProviderName == "" {
			return nil, "", fmt.Errorf("provider_name is required when all_providers=false")
		}
		args = append(args, input.ProviderName)
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
