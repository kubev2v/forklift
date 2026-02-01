package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/discovery"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
)

// MTVWriteInput represents the input for the mtv_write tool.
type MTVWriteInput struct {
	// Command is the kubectl-mtv command to execute (e.g., "create provider", "delete plan", "start plan")
	Command string `json:"command" jsonschema:"kubectl-mtv command path (e.g. create provider, delete plan, patch mapping)"`

	// Args are positional arguments for the command (e.g., resource name)
	Args []string `json:"args,omitempty" jsonschema:"Positional arguments (e.g. resource name to create/delete)"`

	// Flags are command-specific flags as key-value pairs (values can be strings, numbers, or booleans)
	Flags map[string]any `json:"flags,omitempty" jsonschema:"Command flags as key-value pairs (e.g. type: vsphere, url: https://vcenter.example.com, wait: true)"`

	// Namespace is the Kubernetes namespace (shortcut for -n flag)
	Namespace string `json:"namespace,omitempty" jsonschema:"Target Kubernetes namespace"`

	// DryRun shows the command without executing
	DryRun bool `json:"dry_run,omitempty" jsonschema:"Show command without executing (educational mode)"`
}

// GetMTVWriteTool returns the tool definition for read-write MTV commands.
func GetMTVWriteTool(registry *discovery.Registry) *mcp.Tool {
	description := registry.GenerateReadWriteDescription()

	return &mcp.Tool{
		Name:        "mtv_write",
		Description: description,
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":      map[string]any{"type": "string", "description": "The executed command"},
				"return_value": map[string]any{"type": "integer", "description": "Exit code (0 = success)"},
				"data":         map[string]any{"type": "object", "description": "Structured JSON response data"},
				"output":       map[string]any{"type": "string", "description": "Plain text output (when not JSON)"},
				"stderr":       map[string]any{"type": "string", "description": "Error output if any"},
			},
		},
	}
}

// HandleMTVWrite returns a handler function for the mtv_write tool.
func HandleMTVWrite(registry *discovery.Registry) func(context.Context, *mcp.CallToolRequest, MTVWriteInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input MTVWriteInput) (*mcp.CallToolResult, any, error) {
		// Extract K8s credentials from HTTP headers (for SSE mode)
		if req.Extra != nil && req.Extra.Header != nil {
			ctx = util.WithKubeCredsFromHeaders(ctx, req.Extra.Header)
		}

		// Normalize command path
		cmdPath := normalizeCommandPath(input.Command)

		// Validate command exists and is read-write
		if !registry.IsReadWrite(cmdPath) {
			if registry.IsReadOnly(cmdPath) {
				return nil, nil, fmt.Errorf("command '%s' is a read-only operation, use mtv_read tool instead", input.Command)
			}
			// List available commands in error, converting path keys to user-friendly format
			available := registry.ListReadWriteCommands()
			for i, cmd := range available {
				available[i] = strings.ReplaceAll(cmd, "/", " ")
			}
			return nil, nil, fmt.Errorf("unknown command '%s'. Available write commands: %s", input.Command, strings.Join(available, ", "))
		}

		// Enable dry run mode if requested
		if input.DryRun {
			ctx = util.WithDryRun(ctx, true)
		}

		// Build command arguments
		args := buildWriteArgs(cmdPath, input.Args, input.Flags, input.Namespace)

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

// buildWriteArgs builds the command-line arguments for kubectl-mtv write commands.
func buildWriteArgs(cmdPath string, positionalArgs []string, flags map[string]any, namespace string) []string {
	var args []string

	// Add command path parts
	parts := strings.Split(cmdPath, "/")
	args = append(args, parts...)

	// Add positional arguments
	args = append(args, positionalArgs...)

	// Add namespace flag
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Note: Write commands typically don't support -o json output format
	// so we don't add it automatically like we do for read commands

	// Skip set for already handled flags
	skipFlags := map[string]bool{
		"namespace": true, "n": true,
	}

	// Add other flags using the normalizer
	args = appendNormalizedFlags(args, flags, skipFlags)

	return args
}
