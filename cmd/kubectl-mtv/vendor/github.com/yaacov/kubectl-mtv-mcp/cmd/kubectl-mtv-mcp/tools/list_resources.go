package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// ListResourcesInput represents the input for ListResources
type ListResourcesInput struct {
	ResourceType  string `json:"resource_type" jsonschema:"Type of resource to list - 'provider', 'plan', 'mapping', 'host', or 'hook'"`
	Namespace     string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace to query (optional, defaults to current namespace)"`
	AllNamespaces bool   `json:"all_namespaces,omitempty" jsonschema:"List resources across all namespaces"`
	InventoryURL  string `json:"inventory_url,omitempty" jsonschema:"Base URL for inventory service (optional, only used for provider listings to fetch inventory counts)"`
	DryRun        bool   `json:"dry_run,omitempty" jsonschema:"If true, shows commands instead of executing (educational mode)"`
}

// GetListResourcesTool returns the tool definition
func GetListResourcesTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "ListResources",
		Description: `List MTV resources in the cluster.

    Unified tool to list various MTV resource types including providers, plans, mappings, hosts, and hooks.
    This consolidates multiple list operations into a single efficient tool.

    Dry Run Mode: Set dry_run=true to see the command without executing (useful for teaching users)

    Args:
        resource_type: Type of resource to list - 'provider', 'plan', 'mapping', 'host', or 'hook'
        namespace: Kubernetes namespace to query (optional, defaults to current namespace)
        all_namespaces: List resources across all namespaces
        inventory_url: Base URL for inventory service (optional, only used for provider listings to fetch inventory counts)

    Returns:
        JSON formatted resource information

    Examples:
        # List all providers
        ListResources(resource_type="provider")

        # List providers with inventory information
        ListResources(resource_type="provider", inventory_url="https://inventory.example.com")

        # List plans across all namespaces
        ListResources(resource_type="plan", all_namespaces=true)

        # List plans in specific namespace
        ListResources(resource_type="plan", namespace="demo")`,
	}
}

func HandleListResources(ctx context.Context, req *mcp.CallToolRequest, input ListResourcesInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	args := []string{"get"}

	// Validate resource type
	validTypes := []string{"provider", "plan", "mapping", "host", "hook"}
	found := false
	for _, t := range validTypes {
		if input.ResourceType == t {
			found = true
			break
		}
	}
	if !found {
		return nil, "", fmt.Errorf("invalid resource_type '%s'. Valid types: %v", input.ResourceType, validTypes)
	}

	args = append(args, input.ResourceType)
	args = append(args, mtvmcp.BuildBaseArgs(input.Namespace, input.AllNamespaces)...)

	if input.ResourceType == "provider" && input.InventoryURL != "" {
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
