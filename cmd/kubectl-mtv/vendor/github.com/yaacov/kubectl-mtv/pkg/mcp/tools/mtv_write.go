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
	Command string `json:"command" jsonschema:"Command path (e.g. create provider, delete plan, patch mapping)"`

	Flags map[string]any `json:"flags,omitempty" jsonschema:"All parameters including positional args and options (e.g. name: \"my-provider\", type: \"vsphere\", url: \"https://vcenter/sdk\", namespace: \"ns\")"`

	DryRun bool `json:"dry_run,omitempty" jsonschema:"If true, does not execute. Returns the equivalent CLI command in the output field instead"`
}

// GetMTVWriteTool returns the tool definition for read-write MTV commands.
// The input schema (jsonschema tags on MTVWriteInput) already describes parameters.
// The description lists available commands and hints to use mtv_help.
func GetMTVWriteTool(registry *discovery.Registry) *mcp.Tool {
	description := registry.GenerateReadWriteDescription()

	return &mcp.Tool{
		Name:         "mtv_write",
		Description:  description,
		OutputSchema: mtvOutputSchema,
	}
}

// HandleMTVWrite returns a handler function for the mtv_write tool.
func HandleMTVWrite(registry *discovery.Registry) func(context.Context, *mcp.CallToolRequest, MTVWriteInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input MTVWriteInput) (*mcp.CallToolResult, any, error) {
		// Extract K8s credentials from HTTP headers (populated by wrapper in SSE mode)
		ctx = extractKubeCredsFromRequest(ctx, req)

		// Validate input to catch common small-LLM mistakes early
		if err := validateCommandInput(input.Command); err != nil {
			return nil, nil, err
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

		// Build command arguments (all params passed via flags)
		args := buildWriteArgs(cmdPath, input.Flags)

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

		// Check for CLI errors and surface as MCP IsError response
		if errResult := buildCLIErrorResult(data); errResult != nil {
			if cmd := registry.ReadWrite[cmdPath]; cmd != nil {
				enrichErrorWithHelp(errResult, cmd)
			}
			return errResult, nil, nil
		}

		return nil, data, nil
	}
}

// buildWriteArgs builds the command-line arguments for kubectl-mtv write commands.
// All parameters (namespace, name, etc.) are extracted from the flags map.
func buildWriteArgs(cmdPath string, flags map[string]any) []string {
	var args []string

	// Add command path parts
	parts := strings.Split(cmdPath, "/")
	args = append(args, parts...)

	// Extract namespace from flags
	var namespace string
	if flags != nil {
		if v, ok := flags["namespace"]; ok {
			namespace = fmt.Sprintf("%v", v)
		} else if v, ok := flags["n"]; ok {
			namespace = fmt.Sprintf("%v", v)
		}
	}

	// Add namespace flag
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	// Note: Write commands typically don't support --output json output format
	// so we don't add it automatically like we do for read commands

	// Skip set for already handled flags
	skipFlags := map[string]bool{
		"namespace": true, "n": true,
	}

	// Add other flags using the normalizer
	args = appendNormalizedFlags(args, flags, skipFlags)

	return args
}
