package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp/discovery"
)

// MTVReadInput represents the input for the mtv_read tool.
type MTVReadInput struct {
	// Command is the kubectl-mtv command to execute (e.g., "get plan", "get inventory vm", "describe plan")
	Command string `json:"command" jsonschema:"kubectl-mtv command path (e.g. get plan, get inventory vm, describe mapping)"`

	// Args are positional arguments for the command (e.g., plan name, provider name)
	Args []string `json:"args,omitempty" jsonschema:"Positional arguments (e.g. resource name, provider name)"`

	// Flags are command-specific flags as key-value pairs
	Flags map[string]string `json:"flags,omitempty" jsonschema:"Command flags as key-value pairs (e.g. output: json, watch: true)"`

	// Namespace is the Kubernetes namespace (shortcut for -n flag)
	Namespace string `json:"namespace,omitempty" jsonschema:"Target Kubernetes namespace"`

	// AllNamespaces queries across all namespaces (shortcut for -A flag)
	AllNamespaces bool `json:"all_namespaces,omitempty" jsonschema:"Query across all namespaces"`

	// InventoryURL is the base URL for the inventory service
	InventoryURL string `json:"inventory_url,omitempty" jsonschema:"Base URL for inventory service (for provider inventory queries)"`

	// DryRun shows the command without executing
	DryRun bool `json:"dry_run,omitempty" jsonschema:"Show command without executing (educational mode)"`
}

// GetMTVReadTool returns the tool definition for read-only MTV commands.
func GetMTVReadTool(registry *discovery.Registry) *mcp.Tool {
	description := registry.GenerateReadOnlyDescription()

	return &mcp.Tool{
		Name:        "mtv_read",
		Description: description,
	}
}

// HandleMTVRead returns a handler function for the mtv_read tool.
func HandleMTVRead(registry *discovery.Registry) func(context.Context, *mcp.CallToolRequest, MTVReadInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input MTVReadInput) (*mcp.CallToolResult, any, error) {
		// Normalize command path
		cmdPath := normalizeCommandPath(input.Command)

		// Validate command exists and is read-only
		if !registry.IsReadOnly(cmdPath) {
			if registry.IsReadWrite(cmdPath) {
				return nil, nil, fmt.Errorf("command '%s' is a write operation, use mtv_write tool instead", input.Command)
			}
			// List available commands in error
			available := registry.ListReadOnlyCommands()
			return nil, nil, fmt.Errorf("unknown command '%s'. Available read commands: %s", input.Command, strings.Join(available, ", "))
		}

		// Enable dry run mode if requested
		if input.DryRun {
			ctx = mtvmcp.WithDryRun(ctx, true)
		}

		// Build command arguments
		args := buildArgs(cmdPath, input.Args, input.Flags, input.Namespace, input.AllNamespaces, input.InventoryURL)

		// Execute command
		result, err := mtvmcp.RunKubectlMTVCommand(ctx, args)
		if err != nil {
			return nil, nil, fmt.Errorf("command failed: %w", err)
		}

		// Parse and return result
		data, err := mtvmcp.UnmarshalJSONResponse(result)
		if err != nil {
			return nil, nil, err
		}

		return nil, data, nil
	}
}

// normalizeCommandPath converts a command string to a path key.
// "get plan" -> "get/plan"
// "get inventory vm" -> "get/inventory/vm"
func normalizeCommandPath(cmd string) string {
	// Trim and normalize whitespace
	cmd = strings.TrimSpace(cmd)
	parts := strings.Fields(cmd)
	return strings.Join(parts, "/")
}

// buildArgs builds the command-line arguments for kubectl-mtv.
func buildArgs(cmdPath string, positionalArgs []string, flags map[string]string, namespace string, allNamespaces bool, inventoryURL string) []string {
	var args []string

	// Add command path parts
	parts := strings.Split(cmdPath, "/")
	args = append(args, parts...)

	// Add positional arguments
	args = append(args, positionalArgs...)

	// Add namespace flags
	if allNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Add inventory URL if provided
	if inventoryURL != "" {
		args = append(args, "--inventory-url", inventoryURL)
	}

	// Add output format - default to json for MCP
	hasOutput := false
	if flags != nil {
		if _, ok := flags["output"]; ok {
			hasOutput = true
		}
		if _, ok := flags["o"]; ok {
			hasOutput = true
		}
	}
	if !hasOutput {
		args = append(args, "-o", "json")
	}

	// Add other flags
	for name, value := range flags {
		// Skip already handled flags
		if name == "namespace" || name == "n" || name == "all_namespaces" || name == "A" || name == "inventory_url" || name == "inventory-url" || name == "i" {
			continue
		}

		// Handle boolean flags
		if value == "true" {
			args = append(args, "--"+name)
		} else if value == "false" {
			// Skip false boolean flags
			continue
		} else if value != "" {
			// String/int flag with value
			args = append(args, "--"+name, value)
		}
	}

	return args
}
