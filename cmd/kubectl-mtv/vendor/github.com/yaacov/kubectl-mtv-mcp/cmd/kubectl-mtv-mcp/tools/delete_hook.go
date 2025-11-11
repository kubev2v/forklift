package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// DeleteHookInput represents the input for DeleteHook
type DeleteHookInput struct {
	HookName  string `json:"hook_name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	AllHooks  bool   `json:"all_hooks,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

// GetDeleteHookTool returns the tool definition
func GetDeleteHookTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "DeleteHook",
		Description: `Delete one or more migration hooks.

    WARNING: This will remove migration hooks.

    Args:
        hook_name: Name of the hook to delete (required unless all_hooks=True)
        dry_run: If true, shows the kubectl-mtv command instead of executing it (educational mode) (optional, default: false)
        namespace: Kubernetes namespace containing the hook (optional)
        all_hooks: Delete all hooks in the namespace (optional)

    Returns:
        Command output confirming hook deletion

    Examples:
        # Delete specific hook
        DeleteHook(hook_name="pre-migration-check")

        # Delete all hooks in namespace
        DeleteHook(all_hooks=true, namespace="demo")`,
	}
}

func HandleDeleteHook(ctx context.Context, req *mcp.CallToolRequest, input DeleteHookInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	args := []string{"delete", "hook"}

	if input.AllHooks {
		args = append(args, "--all")
	} else {
		if input.HookName == "" {
			return nil, "", fmt.Errorf("hook_name is required when all_hooks=false")
		}
		args = append(args, input.HookName)
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
