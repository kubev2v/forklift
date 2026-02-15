package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
)

// KubectlLogsInput represents the input for the kubectl_logs tool.
// All parameters are passed via flags, consistent with mtv_read/mtv_write tools.
type KubectlLogsInput struct {
	Flags map[string]any `json:"flags,omitempty" jsonschema:"All parameters as key-value pairs (e.g. name: \"deployments/forklift-controller\", namespace: \"openshift-mtv\", filter_plan: \"my-plan\")"`

	DryRun bool `json:"dry_run,omitempty" jsonschema:"If true, does not execute. Returns the equivalent CLI command in the output field instead"`
}

// KubectlInput represents the input for the kubectl tool (get, describe, events).
// All parameters (except action and dry_run) are passed via flags.
type KubectlInput struct {
	Action string `json:"action" jsonschema:"get | describe | events"`

	Flags map[string]any `json:"flags,omitempty" jsonschema:"All parameters as key-value pairs (e.g. resource_type: \"pods\", namespace: \"openshift-mtv\", labels: \"plan=my-plan\")"`

	DryRun bool `json:"dry_run,omitempty" jsonschema:"If true, does not execute. Returns the equivalent CLI command in the output field instead"`
}

// kubectlDebugParams holds the resolved parameters for kubectl debug operations.
// This internal struct is populated from the Flags map.
type kubectlDebugParams struct {
	Name            string
	ResourceType    string
	Namespace       string
	AllNamespaces   bool
	Labels          string
	Container       string
	Previous        bool
	TailLines       int
	Since           string
	Output          string
	FieldSelector   string
	SortBy          string
	ForResource     string
	Grep            string
	IgnoreCase      bool
	NoTimestamps    bool
	LogFormat       string
	FilterPlan      string
	FilterProvider  string
	FilterVM        string
	FilterMigration string
	FilterLevel     string
	FilterLogger    string
}

// resolveDebugParams extracts all parameters from the flags map into a typed struct.
func resolveDebugParams(flags map[string]any) kubectlDebugParams {
	p := kubectlDebugParams{}
	if flags == nil {
		return p
	}
	p.Name = flagStr(flags, "name")
	p.ResourceType = flagStr(flags, "resource_type")
	p.Namespace = flagStr(flags, "namespace")
	p.AllNamespaces = flagBool(flags, "all_namespaces")
	p.Labels = flagStr(flags, "labels")
	p.Container = flagStr(flags, "container")
	p.Previous = flagBool(flags, "previous")
	p.TailLines = flagInt(flags, "tail_lines")
	p.Since = flagStr(flags, "since")
	p.Output = flagStr(flags, "output")
	p.FieldSelector = flagStr(flags, "field_selector")
	p.SortBy = flagStr(flags, "sort_by")
	p.ForResource = flagStr(flags, "for_resource")
	p.Grep = flagStr(flags, "grep")
	p.IgnoreCase = flagBool(flags, "ignore_case")
	p.NoTimestamps = flagBool(flags, "no_timestamps")
	p.LogFormat = flagStr(flags, "log_format")
	p.FilterPlan = flagStr(flags, "filter_plan")
	p.FilterProvider = flagStr(flags, "filter_provider")
	p.FilterVM = flagStr(flags, "filter_vm")
	p.FilterMigration = flagStr(flags, "filter_migration")
	p.FilterLevel = flagStr(flags, "filter_level")
	p.FilterLogger = flagStr(flags, "filter_logger")
	return p
}

// flagStr extracts a string from the flags map.
func flagStr(flags map[string]any, key string) string {
	if v, ok := flags[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// flagBool extracts a boolean from the flags map.
func flagBool(flags map[string]any, key string) bool {
	if v, ok := flags[key]; ok {
		return parseBoolValue(v)
	}
	return false
}

// flagInt extracts an integer from the flags map.
func flagInt(flags map[string]any, key string) int {
	if v, ok := flags[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return 0
}

// GetMinimalKubectlLogsTool returns the minimal tool definition for kubectl log retrieval.
// This tool is focused on forklift-controller logs with JSON parsing and filtering.
func GetMinimalKubectlLogsTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "kubectl_logs",
		Description: `Get logs from any Kubernetes pod or deployment, with extra structured JSON filters for forklift-controller logs.
Use this for debugging migration execution: filter by plan, VM, or error level.

All parameters go in flags.
Required: name (resource-type/name format, e.g. "deployments/forklift-controller" or "pod/my-pod")
Default: last 500 lines. Set tail_lines: -1 for all logs.

Common flags: namespace, container, previous, tail_lines, since, grep, ignore_case.
Log format: log_format (json|text|pretty), no_timestamps.
JSON filters (forklift-controller only): filter_plan, filter_provider, filter_vm,
  filter_migration, filter_level (info|debug|error|warn),
  filter_logger (plan|provider|migration|networkMap|storageMap).

Examples:
  {flags: {name: "deployments/forklift-controller", namespace: "openshift-mtv"}}
  {flags: {name: "deployments/forklift-controller", namespace: "openshift-mtv", filter_plan: "my-plan", filter_level: "error"}}
  {flags: {name: "pod/virt-v2v-cold-xyz", namespace: "target-ns", tail_lines: 100}}`,
		OutputSchema: mtvOutputSchema,
	}
}

// GetMinimalKubectlTool returns the minimal tool definition for kubectl resource inspection.
// This tool handles get, describe, and events actions for Kubernetes resources.
func GetMinimalKubectlTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "kubectl",
		Description: `Inspect standard Kubernetes resources (pods, PVCs, services, deployments, events).
Use ONLY for standard K8s objects. All MTV/Forklift custom resources (plans, providers, mappings, hooks, hosts) go through mtv_read and mtv_write.
For MTV source-provider inventory (VMs, datastores, networks from vSphere/oVirt/OpenStack), use mtv_read "get inventory" commands.

Actions: get, describe, events.
All parameters (except action) go in flags.
Flag names use underscores (e.g. resource_type, all_namespaces, for_resource).

get/describe: resource_type (required), name, labels, output.
events: for_resource, field_selector, sort_by.
Common: namespace, all_namespaces.

Examples:
  {action: "get", flags: {resource_type: "pods", namespace: "openshift-mtv", labels: "plan=my-plan"}}
  {action: "describe", flags: {resource_type: "pvc", name: "my-pvc", namespace: "target-ns"}}
  {action: "events", flags: {for_resource: "pod/virt-v2v-xxx", namespace: "target-ns"}}`,
		OutputSchema: mtvOutputSchema,
	}
}

// HandleKubectlLogs handles the kubectl_logs tool invocation.
func HandleKubectlLogs(ctx context.Context, req *mcp.CallToolRequest, input KubectlLogsInput) (*mcp.CallToolResult, any, error) {
	// Extract K8s credentials from HTTP headers (populated by wrapper in SSE mode)
	ctx = extractKubeCredsFromRequest(ctx, req)

	// Enable dry run mode if requested
	if input.DryRun {
		ctx = util.WithDryRun(ctx, true)
	}

	// Resolve all parameters from the flags map
	p := resolveDebugParams(input.Flags)

	if p.Name == "" {
		return nil, nil, fmt.Errorf("'name' is required in flags (e.g. flags: {name: \"deployments/forklift-controller\"})")
	}

	args := buildLogsArgs(p)

	// Execute kubectl command
	result, err := util.RunKubectlCommand(ctx, args)
	if err != nil {
		return nil, nil, fmt.Errorf("kubectl command failed: %w", err)
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

	// Process logs with filtering and formatting
	if output, ok := data["output"].(string); ok {
		processLogsOutput(data, output, p)
	}

	return nil, data, nil
}

// HandleKubectl handles the kubectl tool invocation (get, describe, events).
func HandleKubectl(ctx context.Context, req *mcp.CallToolRequest, input KubectlInput) (*mcp.CallToolResult, any, error) {
	// Extract K8s credentials from HTTP headers (populated by wrapper in SSE mode)
	ctx = extractKubeCredsFromRequest(ctx, req)

	// Enable dry run mode if requested
	if input.DryRun {
		ctx = util.WithDryRun(ctx, true)
	}

	// Resolve all parameters from the flags map
	p := resolveDebugParams(input.Flags)

	var args []string

	switch input.Action {
	case "get":
		if p.ResourceType == "" {
			return nil, nil, fmt.Errorf("get action requires 'resource_type' in flags (e.g. flags: {resource_type: \"pods\"})")
		}
		args = buildGetArgs(p)
	case "describe":
		if p.ResourceType == "" {
			return nil, nil, fmt.Errorf("describe action requires 'resource_type' in flags (e.g. flags: {resource_type: \"pods\"})")
		}
		args = buildDescribeArgs(p)
	case "events":
		args = buildEventsArgs(p)
	default:
		return nil, nil, fmt.Errorf("unknown action '%s'. Valid actions: get, describe, events", input.Action)
	}

	// Execute kubectl command
	result, err := util.RunKubectlCommand(ctx, args)
	if err != nil {
		return nil, nil, fmt.Errorf("kubectl command failed: %w", err)
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

	return nil, data, nil
}

// processLogsOutput applies grep, JSON detection, filtering, and formatting to log output.
// It modifies the data map in place with the processed results.
func processLogsOutput(data map[string]interface{}, output string, p kubectlDebugParams) {
	// Apply grep filter first (regex pattern matching)
	if p.Grep != "" {
		filtered, err := filterLogsByPattern(output, p.Grep, p.IgnoreCase)
		if err != nil {
			data["warning"] = fmt.Sprintf("grep filter error: %v", err)
			data["output"] = output
			return
		}
		output = filtered
	}

	// Check if logs appear to be JSON formatted by inspecting the first line
	isJSONLogs := looksLikeJSONLogs(output)
	hasFilters := hasJSONParamFilters(p)

	// Warn if JSON filters are requested but logs don't appear to be JSON
	if hasFilters && !isJSONLogs {
		data["warning"] = "JSON filters were specified but logs do not appear to be in JSON format. Filters will be ignored."
	}

	if isJSONLogs {
		// Normalize LogFormat to a valid value before processing
		// Valid formats: "json", "text", "pretty"
		format := p.LogFormat
		switch format {
		case "json", "text", "pretty":
			// Valid format, use as-is
		case "":
			format = "json"
		default:
			// Invalid format specified, default to "json" and warn
			data["warning"] = fmt.Sprintf("Invalid log_format '%s' specified, defaulting to 'json'. Valid formats: json, text, pretty", format)
			format = "json"
		}

		// Apply JSON filtering and formatting for JSON-formatted logs
		normalizedParams := p
		normalizedParams.LogFormat = format
		formatted, err := filterAndFormatJSONLogs(output, normalizedParams)
		if err != nil {
			data["warning"] = fmt.Sprintf("JSON log processing error: %v", err)
			data["output"] = output
			return
		}

		// Set the appropriate output field based on format
		if format == "json" {
			// For JSON format, put parsed entries in "logs" field
			delete(data, "output")
			data["logs"] = formatted
		} else {
			// For text/pretty formats, keep as "output" string
			if str, ok := formatted.(string); ok {
				data["output"] = str
			} else {
				jsonBytes, _ := json.Marshal(formatted)
				data["output"] = string(jsonBytes)
			}
		}
	} else {
		// Non-JSON logs: return as raw text, skip JSON parsing entirely
		data["output"] = output
	}
}

// buildLogsArgs builds arguments for kubectl logs command.
func buildLogsArgs(p kubectlDebugParams) []string {
	args := []string{"logs"}

	if p.Name != "" {
		args = append(args, p.Name)
	}
	if p.Namespace != "" {
		args = append(args, "-n", p.Namespace)
	}
	if p.Container != "" {
		args = append(args, "-c", p.Container)
	}
	if p.Previous {
		args = append(args, "--previous")
	}

	// Tail lines - default to 500 if not specified; -1 gets all logs
	if p.TailLines == 0 {
		args = append(args, "--tail", "500")
	} else if p.TailLines > 0 {
		args = append(args, "--tail", strconv.Itoa(p.TailLines))
	}

	if p.Since != "" {
		args = append(args, "--since", p.Since)
	}

	// Timestamps enabled by default; use no_timestamps=true to disable
	if !p.NoTimestamps {
		args = append(args, "--timestamps")
	}

	return args
}

// buildGetArgs builds arguments for kubectl get command.
func buildGetArgs(p kubectlDebugParams) []string {
	args := []string{"get"}

	if p.ResourceType != "" {
		args = append(args, p.ResourceType)
	}
	if p.Name != "" {
		args = append(args, p.Name)
	}
	if p.AllNamespaces {
		args = append(args, "-A")
	} else if p.Namespace != "" {
		args = append(args, "-n", p.Namespace)
	}
	if p.Labels != "" {
		args = append(args, "-l", p.Labels)
	}

	output := p.Output
	if output == "" {
		output = util.GetOutputFormat()
	}
	if output != "text" {
		args = append(args, "-o", output)
	}

	return args
}

// buildDescribeArgs builds arguments for kubectl describe command.
func buildDescribeArgs(p kubectlDebugParams) []string {
	args := []string{"describe"}

	if p.ResourceType != "" {
		args = append(args, p.ResourceType)
	}
	if p.Name != "" {
		args = append(args, p.Name)
	}
	if p.AllNamespaces {
		args = append(args, "-A")
	} else if p.Namespace != "" {
		args = append(args, "-n", p.Namespace)
	}
	if p.Labels != "" {
		args = append(args, "-l", p.Labels)
	}

	return args
}

// buildEventsArgs builds arguments for kubectl get events command with specialized filtering.
func buildEventsArgs(p kubectlDebugParams) []string {
	args := []string{"get", "events"}

	if p.AllNamespaces {
		args = append(args, "-A")
	} else if p.Namespace != "" {
		args = append(args, "-n", p.Namespace)
	}
	if p.ForResource != "" {
		args = append(args, "--for", p.ForResource)
	}
	if p.FieldSelector != "" {
		args = append(args, "--field-selector", p.FieldSelector)
	}
	if p.SortBy != "" {
		args = append(args, "--sort-by", p.SortBy)
	}

	output := p.Output
	if output == "" {
		output = util.GetOutputFormat()
	}
	if output != "text" {
		args = append(args, "-o", output)
	}

	return args
}

// filterLogsByPattern filters log lines by a regex pattern.
// If pattern is empty, returns the original logs unchanged.
// If ignoreCase is true, the pattern matching is case-insensitive.
func filterLogsByPattern(logs string, pattern string, ignoreCase bool) (string, error) {
	if pattern == "" {
		return logs, nil
	}

	flags := ""
	if ignoreCase {
		flags = "(?i)"
	}

	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return "", fmt.Errorf("invalid grep pattern: %w", err)
	}

	var filtered []string
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if re.MatchString(line) {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n"), nil
}

// JSONLogEntry represents a structured log entry from forklift controller.
// The forklift controller outputs JSON logs with fields like:
// {"level":"info","ts":"2026-02-05 10:45:52","logger":"plan|zw4bt","msg":"Reconcile started.","plan":{"name":"my-plan","namespace":"demo"}}
type JSONLogEntry struct {
	Level     string            `json:"level"`
	Ts        string            `json:"ts"`
	Logger    string            `json:"logger"`
	Msg       string            `json:"msg"`
	Plan      map[string]string `json:"plan,omitempty"`
	Provider  map[string]string `json:"provider,omitempty"`
	Map       map[string]string `json:"map,omitempty"`
	Migration map[string]string `json:"migration,omitempty"`
	VM        string            `json:"vm,omitempty"`
	VMName    string            `json:"vmName,omitempty"`
	VMID      string            `json:"vmID,omitempty"`
	ReQ       int               `json:"reQ,omitempty"`
}

// RawLogLine represents a log line that could not be parsed as JSON.
// Used to preserve malformed or non-JSON log lines in the output.
type RawLogLine struct {
	Raw string `json:"raw"`
}

// hasJSONParamFilters returns true if any JSON-specific filters are set.
func hasJSONParamFilters(p kubectlDebugParams) bool {
	return p.FilterPlan != "" ||
		p.FilterProvider != "" ||
		p.FilterVM != "" ||
		p.FilterMigration != "" ||
		p.FilterLevel != "" ||
		p.FilterLogger != ""
}

// looksLikeJSONLogs checks if the logs appear to be in JSON format by examining up to 5 non-empty lines.
// It handles the kubectl --timestamps prefix (e.g., "2026-02-05T10:45:52.123Z {\"level\":...}")
// Returns true as soon as any scanned line contains valid JSON with expected log fields (level, msg).
// Returns false if none of the scanned lines yield a valid JSON entry.
func looksLikeJSONLogs(logs string) bool {
	lines := strings.Split(logs, "\n")

	// Check up to 5 non-empty lines for JSON format
	const maxLinesToCheck = 5
	checkedLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		checkedLines++
		if checkedLines > maxLinesToCheck {
			break
		}

		// Extract JSON part (skip timestamp prefix if present)
		idx := strings.Index(trimmed, "{")
		if idx < 0 {
			// No JSON object found in this line, try next
			continue
		}
		jsonPart := trimmed[idx:]

		// Try to parse as JSON
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(jsonPart), &entry); err != nil {
			// Not valid JSON, try next line
			continue
		}

		// Check for expected JSON log fields (forklift controller format)
		// A valid JSON log should have at least "level" and "msg" fields
		_, hasLevel := entry["level"]
		_, hasMsg := entry["msg"]

		if hasLevel && hasMsg {
			return true
		}
	}

	return false
}

// matchesParamFilters checks if a log entry matches all specified filters.
func matchesParamFilters(entry JSONLogEntry, p kubectlDebugParams) bool {
	if p.FilterLevel != "" && !strings.EqualFold(entry.Level, p.FilterLevel) {
		return false
	}
	if p.FilterLogger != "" {
		loggerType := strings.Split(entry.Logger, "|")[0]
		if !strings.EqualFold(loggerType, p.FilterLogger) {
			return false
		}
	}
	if p.FilterPlan != "" {
		planName := ""
		if entry.Plan != nil {
			planName = entry.Plan["name"]
		}
		if !strings.EqualFold(planName, p.FilterPlan) {
			return false
		}
	}
	if p.FilterProvider != "" {
		providerName := ""
		if entry.Provider != nil {
			providerName = entry.Provider["name"]
		}
		if !strings.EqualFold(providerName, p.FilterProvider) {
			return false
		}
	}
	if p.FilterVM != "" {
		vmMatch := strings.EqualFold(entry.VM, p.FilterVM) ||
			strings.EqualFold(entry.VMName, p.FilterVM) ||
			strings.EqualFold(entry.VMID, p.FilterVM)
		if !vmMatch {
			return false
		}
	}
	if p.FilterMigration != "" {
		loggerParts := strings.Split(entry.Logger, "|")
		loggerType := loggerParts[0]
		if loggerType != "migration" {
			return false
		}
		migrationName := ""
		if entry.Migration != nil {
			migrationName = entry.Migration["name"]
		}
		if migrationName == "" && len(loggerParts) > 1 {
			migrationName = loggerParts[1]
		}
		if !strings.EqualFold(migrationName, p.FilterMigration) {
			return false
		}
	}
	return true
}

// filterAndFormatJSONLogs parses JSON logs, applies filters, and formats output.
// It returns the processed logs based on the specified format:
// - "json": Array of mixed JSONLogEntry and RawLogLine (for malformed lines)
// - "text": Original raw JSONL lines (filtered)
// - "pretty": Human-readable formatted output
func filterAndFormatJSONLogs(logs string, p kubectlDebugParams) (interface{}, error) {
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return []interface{}{}, nil
	}

	var logLines []interface{}
	var filteredLines []string
	hasFilters := hasJSONParamFilters(p)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle timestamp prefix from kubectl --timestamps flag
		// Format: "2026-02-05T10:45:52.123456789Z {"level":"info",...}"
		jsonPart := line
		timestampPrefix := ""
		if idx := strings.Index(line, "{"); idx > 0 {
			timestampPrefix = line[:idx]
			jsonPart = line[idx:]
		}

		var entry JSONLogEntry
		if err := json.Unmarshal([]byte(jsonPart), &entry); err != nil {
			if !hasFilters {
				logLines = append(logLines, RawLogLine{Raw: line})
				filteredLines = append(filteredLines, line)
			}
			continue
		}

		if hasFilters && !matchesParamFilters(entry, p) {
			continue
		}

		logLines = append(logLines, entry)
		filteredLines = append(filteredLines, timestampPrefix+jsonPart)
	}

	format := p.LogFormat
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		return logLines, nil
	case "text":
		return strings.Join(filteredLines, "\n"), nil
	case "pretty":
		return formatPrettyLogs(logLines), nil
	default:
		return logLines, nil
	}
}

// formatPrettyLogs formats log entries in a human-readable format.
// It handles both JSONLogEntry (parsed) and RawLogLine (malformed) types.
func formatPrettyLogs(logLines []interface{}) string {
	var lines []string
	for _, item := range logLines {
		switch v := item.(type) {
		case RawLogLine:
			// Include raw malformed lines as-is
			lines = append(lines, v.Raw)
		case JSONLogEntry:
			// Format: [LEVEL] timestamp logger: message (context)
			levelUpper := strings.ToUpper(v.Level)
			context := ""

			// Add context info
			if v.Plan != nil && v.Plan["name"] != "" {
				context = fmt.Sprintf(" plan=%s", v.Plan["name"])
				if ns := v.Plan["namespace"]; ns != "" {
					context += fmt.Sprintf("/%s", ns)
				}
			} else if v.Provider != nil && v.Provider["name"] != "" {
				context = fmt.Sprintf(" provider=%s", v.Provider["name"])
				if ns := v.Provider["namespace"]; ns != "" {
					context += fmt.Sprintf("/%s", ns)
				}
			} else if v.Map != nil && v.Map["name"] != "" {
				context = fmt.Sprintf(" map=%s", v.Map["name"])
				if ns := v.Map["namespace"]; ns != "" {
					context += fmt.Sprintf("/%s", ns)
				}
			}

			if v.VM != "" {
				context += fmt.Sprintf(" vm=%s", v.VM)
			} else if v.VMName != "" {
				context += fmt.Sprintf(" vm=%s", v.VMName)
			}

			line := fmt.Sprintf("[%s] %s %s: %s%s", levelUpper, v.Ts, v.Logger, v.Msg, context)
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
