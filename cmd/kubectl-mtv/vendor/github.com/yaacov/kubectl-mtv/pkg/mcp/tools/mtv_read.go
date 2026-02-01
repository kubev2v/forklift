package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/discovery"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
)

// MTVReadInput represents the input for the mtv_read tool.
type MTVReadInput struct {
	// Command is the kubectl-mtv command to execute (e.g., "get plan", "get inventory vm", "describe plan")
	Command string `json:"command" jsonschema:"kubectl-mtv command path (e.g. get plan, get inventory vm, describe mapping)"`

	// Args are positional arguments for the command (e.g., plan name, provider name)
	Args []string `json:"args,omitempty" jsonschema:"Positional arguments (e.g. resource name, provider name)"`

	// Flags are command-specific flags as key-value pairs (values can be strings, numbers, or booleans)
	Flags map[string]any `json:"flags,omitempty" jsonschema:"Command flags as key-value pairs (e.g. output: json, watch: true)"`

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
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":      map[string]any{"type": "string", "description": "The executed command"},
				"return_value": map[string]any{"type": "integer", "description": "Exit code (0 = success)"},
				"data": map[string]any{
					"description": "Structured JSON response data (object or array)",
					"oneOf": []map[string]any{
						{"type": "object"},
						{"type": "array"},
					},
				},
				"output": map[string]any{"type": "string", "description": "Plain text output (when not JSON)"},
				"stderr": map[string]any{"type": "string", "description": "Error output if any"},
			},
		},
	}
}

// HandleMTVRead returns a handler function for the mtv_read tool.
func HandleMTVRead(registry *discovery.Registry) func(context.Context, *mcp.CallToolRequest, MTVReadInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input MTVReadInput) (*mcp.CallToolResult, any, error) {
		// Extract K8s credentials from HTTP headers (for SSE mode)
		if req.Extra != nil && req.Extra.Header != nil {
			ctx = util.WithKubeCredsFromHeaders(ctx, req.Extra.Header)
		}

		// Normalize command path
		cmdPath := normalizeCommandPath(input.Command)

		// Validate command exists and is read-only
		if !registry.IsReadOnly(cmdPath) {
			if registry.IsReadWrite(cmdPath) {
				return nil, nil, fmt.Errorf("command '%s' is a write operation, use mtv_write tool instead", input.Command)
			}
			// List available commands in error, converting path keys to user-friendly format
			available := registry.ListReadOnlyCommands()
			for i, cmd := range available {
				available[i] = strings.ReplaceAll(cmd, "/", " ")
			}
			return nil, nil, fmt.Errorf("unknown command '%s'. Available read commands: %s", input.Command, strings.Join(available, ", "))
		}

		// Enable dry run mode if requested
		if input.DryRun {
			ctx = util.WithDryRun(ctx, true)
		}

		// Build command arguments
		args := buildArgs(cmdPath, input.Args, input.Flags, input.Namespace, input.AllNamespaces, input.InventoryURL)

		// Execute command
		result, err := util.RunKubectlMTVCommand(ctx, args)
		if err != nil {
			return nil, nil, fmt.Errorf("command failed: %w", err)
		}

		// Parse and return result
		data, err := util.UnmarshalJSONResponse(result)
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
func buildArgs(cmdPath string, positionalArgs []string, flags map[string]any, namespace string, allNamespaces bool, inventoryURL string) []string {
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

	// Add output format - use configured default from MCP server
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
		format := util.GetOutputFormat()
		// For "text" format, don't add -o flag to use default table output
		if format != "text" {
			args = append(args, "-o", format)
		}
	}

	// Skip set for already handled flags (namespace, output, inventory-url variants)
	skipFlags := map[string]bool{
		"namespace": true, "n": true,
		"all_namespaces": true, "A": true,
		"inventory_url": true, "inventory-url": true, "i": true,
		"output": true, "o": true,
	}

	// Add other flags using the normalizer
	args = appendNormalizedFlags(args, flags, skipFlags)

	return args
}

// appendNormalizedFlags appends flags from a map[string]any to the args slice.
// It handles different value types:
//   - bool true: includes the flag with no value (presence flag)
//   - bool false: explicitly passes --flag=false (needed for flags that default to true)
//   - string "true"/"false": treated as boolean
//   - string/number: converted to string form
//
// Flag prefix is determined by key length: single char uses "-x", multi-char uses "--long"
func appendNormalizedFlags(args []string, flags map[string]any, skipFlags map[string]bool) []string {
	for name, value := range flags {
		// Skip flags in the skip set
		if skipFlags != nil && skipFlags[name] {
			continue
		}

		// Determine flag prefix: single dash for single-char flags, double dash for multi-char
		prefix := "--"
		if len(name) == 1 {
			prefix = "-"
		}

		// Handle different value types
		switch v := value.(type) {
		case bool:
			if v {
				// Boolean true: include flag with no value
				args = append(args, prefix+name)
			} else {
				// Boolean false: explicitly pass --flag=false
				// This is needed for flags that default to true (e.g., --migrate-shared-disks)
				args = append(args, prefix+name+"=false")
			}
		case string:
			// Handle string "true"/"false" as boolean for backwards compatibility
			if v == "true" {
				args = append(args, prefix+name)
			} else if v == "false" {
				// Explicitly pass --flag=false for flags that default to true
				args = append(args, prefix+name+"=false")
			} else if v != "" {
				args = append(args, prefix+name, v)
			}
		case float64:
			// JSON numbers are decoded as float64
			// Check if it's a whole number to avoid unnecessary decimals
			if v == float64(int64(v)) {
				args = append(args, prefix+name, fmt.Sprintf("%d", int64(v)))
			} else {
				args = append(args, prefix+name, fmt.Sprintf("%g", v))
			}
		case int, int64, int32:
			args = append(args, prefix+name, fmt.Sprintf("%d", v))
		default:
			// For any other type, convert to string
			if v != nil {
				args = append(args, prefix+name, fmt.Sprintf("%v", v))
			}
		}
	}

	return args
}
