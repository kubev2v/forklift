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
	Command string `json:"command" jsonschema:"Command path (e.g. get plan, get inventory vm, describe mapping)"`

	Flags map[string]any `json:"flags,omitempty" jsonschema:"All parameters including positional args and options (e.g. name: \"my-plan\", provider: \"my-vsphere\", output: \"json\", namespace: \"ns\", query: \"where cpuCount > 4\")"`

	DryRun bool `json:"dry_run,omitempty" jsonschema:"If true, does not execute. Returns the equivalent CLI command in the output field instead"`

	Fields []string `json:"fields,omitempty" jsonschema:"Limit JSON to these top-level keys only (e.g. [name, id, concerns])"`
}

// mtvOutputSchema is the shared output schema for mtv_read and mtv_write tools.
// The "command" field is intentionally omitted to prevent small LLMs from
// mimicking CLI command syntax (e.g., generating "kubectl-mtv get plan ...")
// instead of structured {command, flags} tool calls.
var mtvOutputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"return_value": map[string]any{"type": "integer", "description": "Exit code (0=success)"},
		"data": map[string]any{
			"description": "Response data (object or array)",
			"oneOf": []map[string]any{
				{"type": "object"},
				{"type": "array"},
			},
		},
		"output": map[string]any{"type": "string", "description": "Text output"},
		"stderr": map[string]any{"type": "string", "description": "Error output"},
	},
}

// GetMTVReadTool returns the tool definition for read-only MTV commands.
func GetMTVReadTool(registry *discovery.Registry) *mcp.Tool {
	description := registry.GenerateReadOnlyDescription()

	return &mcp.Tool{
		Name:         "mtv_read",
		Description:  description,
		OutputSchema: mtvOutputSchema,
	}
}

// GetMinimalMTVReadTool returns a minimal tool definition for read-only MTV commands.
// The input schema (jsonschema tags on MTVReadInput) already describes parameters.
// The description only lists available commands and a hint to use mtv_help.
func GetMinimalMTVReadTool(registry *discovery.Registry) *mcp.Tool {
	description := registry.GenerateMinimalReadOnlyDescription()

	return &mcp.Tool{
		Name:         "mtv_read",
		Description:  description,
		OutputSchema: mtvOutputSchema,
	}
}

// GetUltraMinimalMTVReadTool returns the smallest possible tool definition for read-only
// MTV commands, optimized for very small models (< 8B parameters).
// Lists only the most common commands, 2 examples, and omits flags/workflow/notes.
func GetUltraMinimalMTVReadTool(registry *discovery.Registry) *mcp.Tool {
	description := registry.GenerateUltraMinimalReadOnlyDescription()

	return &mcp.Tool{
		Name:         "mtv_read",
		Description:  description,
		OutputSchema: mtvOutputSchema,
	}
}

// validateCommandInput checks for common malformed input patterns from small LLMs.
// When a model sends garbled input (CLI commands, embedded tool results, etc.),
// this returns a corrective error message that teaches the model the right format,
// rather than letting it spiral into a degenerate loop.
func validateCommandInput(command string) error {
	command = strings.TrimSpace(command)

	// Detect full CLI commands pasted as the command field
	lower := strings.ToLower(command)
	if strings.HasPrefix(lower, "kubectl-mtv ") || strings.HasPrefix(lower, "kubectl ") {
		return fmt.Errorf(
			"the 'command' field should be a subcommand path like 'get plan' or 'get inventory vm', " +
				"not a full CLI command. Remove the 'kubectl-mtv' or 'kubectl' prefix")
	}

	// Detect embedded JSON/output (hallucinated tool responses mixed into input)
	if strings.Contains(command, "\"output\"") ||
		strings.Contains(command, "\"return_value\"") ||
		strings.Contains(command, "\"stdout\"") ||
		strings.Contains(command, "[TOOL_CALLS]") {
		return fmt.Errorf(
			"the 'command' field contains what looks like a previous tool response. " +
				"It should only contain a command path like 'get inventory datastore'")
	}

	// Detect overly long command strings (almost certainly malformed)
	if len(command) > 200 {
		return fmt.Errorf(
			"the 'command' field is too long (%d chars). "+
				"It should be a short command path like 'get plan' or 'get inventory vm'",
			len(command))
	}

	return nil
}

// HandleMTVRead returns a handler function for the mtv_read tool.
func HandleMTVRead(registry *discovery.Registry) func(context.Context, *mcp.CallToolRequest, MTVReadInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input MTVReadInput) (*mcp.CallToolResult, any, error) {
		// Extract K8s credentials from HTTP headers (populated by wrapper in SSE mode)
		ctx = extractKubeCredsFromRequest(ctx, req)

		// Validate input to catch common small-LLM mistakes early
		if err := validateCommandInput(input.Command); err != nil {
			return nil, nil, err
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

		// Build command arguments (all params passed via flags)
		args := buildArgs(cmdPath, input.Flags)

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
			return errResult, nil, nil
		}

		// Apply field filtering if requested
		if len(input.Fields) > 0 {
			data = filterResponseFields(data, input.Fields)
		}

		return nil, data, nil
	}
}

// filterResponseFields filters the "data" field of a response to include only the specified fields.
// It handles both array responses ([]interface{}) and single object responses (map[string]interface{}).
// Envelope fields (return_value, stderr, output) are always preserved.
func filterResponseFields(data map[string]interface{}, fields []string) map[string]interface{} {
	if len(fields) == 0 {
		return data
	}

	// Build a set of allowed field names for fast lookup
	allowed := make(map[string]bool, len(fields))
	for _, f := range fields {
		allowed[f] = true
	}

	// Filter the "data" field only, preserving envelope fields
	rawData, ok := data["data"]
	if !ok {
		return data
	}

	switch items := rawData.(type) {
	case []interface{}:
		// Array of items: filter each item
		filtered := make([]interface{}, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				filtered = append(filtered, filterMapFields(m, allowed))
			} else {
				// Non-object items are kept as-is
				filtered = append(filtered, item)
			}
		}
		data["data"] = filtered
	case map[string]interface{}:
		// Single object: filter its fields
		data["data"] = filterMapFields(items, allowed)
	}

	return data
}

// filterMapFields returns a new map containing only keys present in the allowed set.
func filterMapFields(m map[string]interface{}, allowed map[string]bool) map[string]interface{} {
	result := make(map[string]interface{}, len(allowed))
	for key, val := range m {
		if allowed[key] {
			result[key] = val
		}
	}
	return result
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
// All parameters (namespace, all_namespaces, inventory_url, output, name, provider, etc.)
// are extracted from the flags map — there are no separate top-level fields.
func buildArgs(cmdPath string, flags map[string]any) []string {
	var args []string

	// Add command path parts
	parts := strings.Split(cmdPath, "/")
	args = append(args, parts...)

	// Extract namespace / all_namespaces from flags
	var namespace string
	var allNamespaces bool
	if flags != nil {
		if v, ok := flags["namespace"]; ok {
			namespace = fmt.Sprintf("%v", v)
		} else if v, ok := flags["n"]; ok {
			namespace = fmt.Sprintf("%v", v)
		}
		if v, ok := flags["all_namespaces"]; ok {
			allNamespaces = parseBoolValue(v)
		} else if v, ok := flags["A"]; ok {
			allNamespaces = parseBoolValue(v)
		}
	}

	// Add namespace flags
	if allNamespaces {
		args = append(args, "--all-namespaces")
	} else if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	// Extract inventory URL from flags
	var inventoryURL string
	if flags != nil {
		if v, ok := flags["inventory_url"]; ok {
			inventoryURL = fmt.Sprintf("%v", v)
		} else if v, ok := flags["inventory-url"]; ok {
			inventoryURL = fmt.Sprintf("%v", v)
		} else if v, ok := flags["i"]; ok {
			inventoryURL = fmt.Sprintf("%v", v)
		}
	}
	if inventoryURL != "" {
		args = append(args, "--inventory-url", inventoryURL)
	}

	// Add output format - prefer user-specified, then configured default
	var userOutput string
	if flags != nil {
		if v, ok := flags["output"]; ok {
			userOutput = fmt.Sprintf("%v", v)
		} else if v, ok := flags["o"]; ok {
			userOutput = fmt.Sprintf("%v", v)
		}
	}
	if userOutput != "" {
		// User explicitly requested an output format
		args = append(args, "--output", userOutput)
	} else {
		// Use configured default from MCP server
		format := util.GetOutputFormat()
		// For "text" format, don't add --output flag to use default table output
		if format != "text" {
			args = append(args, "--output", format)
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

// buildCLIErrorResult checks if a CLI response indicates failure (non-zero return_value)
// and returns an MCP CallToolResult with IsError=true if so.
// This gives the LLM immediate, unambiguous error feedback instead of embedding
// errors in a "successful" response where they may be overlooked.
// Returns nil if the command succeeded (return_value == 0).
func buildCLIErrorResult(data map[string]interface{}) *mcp.CallToolResult {
	rv, ok := data["return_value"].(float64)
	if !ok || rv == 0 {
		return nil
	}

	errMsg := fmt.Sprintf("Command failed (exit %d)", int(rv))
	if stderr, ok := data["stderr"].(string); ok && stderr != "" {
		errMsg += ": " + strings.TrimSpace(stderr)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: errMsg}},
		IsError: true,
	}
}

// parseBoolValue interprets a value from the flags map as a boolean.
// It handles bool, string ("true"/"True"/"TRUE"/"1"/"false"/"False"/"FALSE"/"0"),
// and float64 (JSON numbers: 1 = true, 0 = false).
func parseBoolValue(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		if strings.EqualFold(val, "true") || val == "1" {
			return true
		}
		return false
	case float64:
		return val != 0
	default:
		return false
	}
}
