package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// QueryLogsInput represents the input for QueryLogs
type QueryLogsInput struct {
	// Pod Selection (use one method)
	PodName       string `json:"pod_name,omitempty" jsonschema:"Explicit pod name"`
	Component     string `json:"component,omitempty" jsonschema:"Component type (controller, api, cdi-operator, etc)"`
	LabelSelector string `json:"label_selector,omitempty" jsonschema:"Label selector (e.g., 'app=forklift')"`
	VMName        string `json:"vm_name,omitempty" jsonschema:"VM name (uses vm.kubevirt.io/name label)"`

	// Context
	Namespace     string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace (auto-detected for components)"`
	AllNamespaces bool   `json:"all_namespaces,omitempty" jsonschema:"Search across all namespaces"`
	Container     string `json:"container,omitempty" jsonschema:"Container name (optional)"`

	// Log Range
	TailLines    int    `json:"tail_lines,omitempty" jsonschema:"Number of recent lines (default 100)"`
	HeadLines    int    `json:"head_lines,omitempty" jsonschema:"Number of first lines"`
	SinceSeconds int    `json:"since_seconds,omitempty" jsonschema:"Logs from last N seconds"`
	SinceTime    string `json:"since_time,omitempty" jsonschema:"Logs since timestamp (RFC3339)"`

	// Filtering
	GrepPattern    string `json:"grep_pattern,omitempty" jsonschema:"Pattern to match (regex)"`
	GrepInvert     bool   `json:"grep_invert,omitempty" jsonschema:"Invert match (exclude matching lines)"`
	GrepIgnoreCase bool   `json:"grep_ignore_case,omitempty" jsonschema:"Case-insensitive matching"`
	GrepContext    int    `json:"grep_context,omitempty" jsonschema:"Lines of context around matches"`

	// Output Control
	MaxLines   int  `json:"max_lines,omitempty" jsonschema:"Maximum lines to return (default 1000)"`
	MaxBytes   int  `json:"max_bytes,omitempty" jsonschema:"Maximum bytes to return (default 51200)"`
	Previous   bool `json:"previous,omitempty" jsonschema:"Get logs from previous container instance"`
	Timestamps bool `json:"timestamps,omitempty" jsonschema:"Include timestamps in output"`

	// Advanced (migration-specific)
	PlanID      string `json:"plan_id,omitempty" jsonschema:"Plan UUID for importer pod lookup"`
	MigrationID string `json:"migration_id,omitempty" jsonschema:"Migration UUID for importer pod lookup"`
	VMID        string `json:"vm_id,omitempty" jsonschema:"VM ID for importer pod lookup"`

	DryRun bool `json:"dry_run,omitempty" jsonschema:"Show commands instead of executing"`
}

// GetQueryLogsTool returns the tool definition
func GetQueryLogsTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "QueryLogs",
		Description: `Query and filter logs from MTV/KubeVirt pods with advanced filtering.

    Unified tool that combines pod discovery, log retrieval, and filtering.
    Designed for AI agents to get relevant log data without overwhelming output.

    POD SELECTION METHODS (use one):
    1. pod_name: Direct pod name
    2. component: Component type (controller, api, cdi-operator, virt-api, etc)
    3. label_selector: Custom label selector
    4. vm_name: VM name (uses vm.kubevirt.io/name label)
    5. plan_id + migration_id + vm_id: For importer pods

    FILTERING FEATURES:
    - Grep: Pattern matching with regex, case-insensitive, invert, context lines
    - Range: Head (first N), Tail (last N), Since (time-based)
    - Limits: Max lines (default 1000), max bytes (default 50KB)
    - Output: Metadata shows what was filtered and why

    DATA REDUCTION:
    Without filtering: Returns 100+ lines (~8KB, ~2000 tokens)
    With grep filtering: Returns 5-20 lines (~1KB, ~250 tokens)
    Reduction: 80-95% less data for AI agents!

    COMMON PATTERNS:

    Error search (most useful for troubleshooting):
        QueryLogs(component="controller", grep_pattern="error|warning", 
                  grep_ignore_case=true, grep_context=3, tail_lines=500)

    Recent logs only:
        QueryLogs(component="api", since_seconds=300, grep_pattern="error")

    First startup logs:
        QueryLogs(component="operator", head_lines=100)

    Migration debugging:
        QueryLogs(label_selector="plan=<uuid>", namespace="demo",
                  grep_pattern="progress|error", tail_lines=200)

    VM troubleshooting:
        QueryLogs(vm_name="my-vm", namespace="production",
                  container="compute", grep_pattern="error", since_seconds=600)

    COMPONENT TYPES (same as QueryPods):
    Forklift: controller, operator, api, validation, ui-plugin, ova-proxy, forklift
    CDI: cdi-operator, cdi-controller, cdi-apiserver, cdi-uploadproxy, cdi
    KubeVirt: virt-operator, virt-api, virt-controller, virt-handler, virt-exportproxy

    Args:
        pod_name: Explicit pod name (optional)
        component: Component type (optional)
        label_selector: Label selector (optional)
        vm_name: VM name (optional)
        namespace: Namespace (auto-detected for components)
        container: Container name (optional)
        tail_lines: Recent N lines (default 100)
        head_lines: First N lines (optional)
        since_seconds: Logs from last N seconds (optional)
        grep_pattern: Pattern to match (optional)
        grep_ignore_case: Case-insensitive (default false)
        grep_invert: Exclude matches (default false)
        grep_context: Context lines around matches (default 0)
        max_lines: Maximum output lines (default 1000)
        max_bytes: Maximum output bytes (default 50KB)
        previous: Previous container instance (default false)
        timestamps: Include timestamps (default false)

    Returns:
        JSON with logs and metadata:
        {
            "pod": {
                "name": "pod-name",
                "namespace": "namespace",
                "container": "container-name"
            },
            "logs": "filtered log content...",
            "metadata": {
                "total_lines": 5234,
                "returned_lines": 127,
                "grep_matches": 127,
                "truncated": false,
                "filters_applied": ["grep", "context", "tail"]
            }
        }

    Examples:
        # Find errors in controller
        QueryLogs(component="controller", grep_pattern="error", grep_ignore_case=true)

        # Recent errors with context
        QueryLogs(component="controller", since_seconds=300, 
                  grep_pattern="error|warning", grep_context=5)

        # Importer pod progress
        QueryLogs(label_selector="vmID=vm-47", namespace="demo",
                  grep_pattern="progress|%", tail_lines=50)

        # VM console logs
        QueryLogs(vm_name="my-vm", namespace="production", 
                  container="compute", tail_lines=100)

        # Startup issues
        QueryLogs(component="operator", previous=true, grep_pattern="fatal|panic")`,
	}
}

func HandleQueryLogs(ctx context.Context, req *mcp.CallToolRequest, input QueryLogsInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Set defaults
	if input.TailLines == 0 && input.HeadLines == 0 && input.SinceSeconds == 0 && input.SinceTime == "" {
		input.TailLines = 100 // Default to last 100 lines
	}
	if input.MaxLines == 0 {
		input.MaxLines = 1000 // Default max lines
	}
	if input.MaxBytes == 0 {
		input.MaxBytes = 51200 // Default 50KB
	}

	// Discover pod if needed
	podName := input.PodName
	namespace := input.Namespace
	var err error

	if podName == "" {
		podName, namespace, err = discoverPod(ctx, input)
		if err != nil {
			return nil, "", fmt.Errorf("failed to discover pod: %w", err)
		}
	}

	// Build kubectl logs command
	args := []string{"logs"}

	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	args = append(args, podName)

	if input.Container != "" {
		args = append(args, "-c", input.Container)
	}

	// Add log retrieval options (kubectl handles these)
	if input.TailLines > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", input.TailLines))
	}
	if input.SinceSeconds > 0 {
		args = append(args, "--since", fmt.Sprintf("%ds", input.SinceSeconds))
	}
	if input.SinceTime != "" {
		args = append(args, "--since-time", input.SinceTime)
	}
	if input.Previous {
		args = append(args, "--previous")
	}
	if input.Timestamps {
		args = append(args, "--timestamps")
	}

	// Get logs
	output, err := mtvmcp.RunKubectlCommand(ctx, args)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get logs: %w", err)
	}

	logs := mtvmcp.ExtractStdoutFromResponse(output)

	// Apply client-side filtering
	filter := mtvmcp.LogFilter{
		GrepPattern:    input.GrepPattern,
		GrepInvert:     input.GrepInvert,
		GrepIgnoreCase: input.GrepIgnoreCase,
		GrepContext:    input.GrepContext,
		HeadLines:      input.HeadLines,
		MaxLines:       input.MaxLines,
		MaxBytes:       input.MaxBytes,
	}

	filteredLogs, metadata := mtvmcp.FilterLogs(logs, filter)

	// Build result
	result := map[string]interface{}{
		"pod": map[string]interface{}{
			"name":      podName,
			"namespace": namespace,
			"container": input.Container,
		},
		"logs":     filteredLogs,
		"metadata": metadata,
	}

	return nil, result, nil
}

// discoverPod finds a pod based on input criteria
func discoverPod(ctx context.Context, input QueryLogsInput) (string, string, error) {
	// Handle importer pod discovery (requires PVC lookup)
	if input.PlanID != "" || input.MigrationID != "" || input.VMID != "" {
		if input.PlanID == "" || input.MigrationID == "" || input.VMID == "" {
			return "", "", fmt.Errorf("for importer logs, plan_id, migration_id, and vm_id are all required")
		}
		return findImporterPod(ctx, input.Namespace, input.PlanID, input.MigrationID, input.VMID)
	}

	// Handle VM name lookup
	if input.VMName != "" {
		input.LabelSelector = fmt.Sprintf("vm.kubevirt.io/name=%s", input.VMName)
	}

	// Use QueryPods logic to find pod
	queryInput := QueryPodsInput{
		Component:     input.Component,
		Namespace:     input.Namespace,
		AllNamespaces: input.AllNamespaces,
		LabelSelector: input.LabelSelector,
	}

	_, result, err := HandleQueryPods(ctx, nil, queryInput)
	if err != nil {
		return "", "", err
	}

	// Extract first pod from results
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected result format from pod query")
	}

	pods, ok := resultMap["pods"].([]interface{})
	if !ok || len(pods) == 0 {
		return "", "", fmt.Errorf("no pods found matching criteria")
	}

	// Get first pod
	pod, ok := pods[0].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected pod format")
	}

	metadata, ok := pod["metadata"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("pod metadata not found")
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return "", "", fmt.Errorf("pod name not found")
	}

	ns, _ := metadata["namespace"].(string)

	// If container not specified but pod has multiple containers, try to guess
	if input.Container == "" {
		if containers, ok := pod["_containers"].([]map[string]interface{}); ok && len(containers) > 1 {
			// For controller, prefer "main" container
			if input.Component == "controller" {
				for _, c := range containers {
					if cName, ok := c["name"].(string); ok && cName == "main" {
						input.Container = "main"
						break
					}
				}
			}
		}
	}

	return name, ns, nil
}

// findImporterPod finds an importer pod via PVC annotation lookup
func findImporterPod(ctx context.Context, namespace, planID, migrationID, vmID string) (string, string, error) {
	if namespace == "" {
		return "", "", fmt.Errorf("namespace is required for importer pod lookup")
	}

	// Find PVCs with migration labels
	labelSelector := fmt.Sprintf("plan=%s,migration=%s,vmID=%s", planID, migrationID, vmID)
	args := []string{"get", "pvc", "-n", namespace, "-l", labelSelector, "-o", "json"}

	output, err := mtvmcp.RunKubectlCommand(ctx, args)
	if err != nil {
		return "", "", fmt.Errorf("failed to get PVCs: %w", err)
	}

	stdout := mtvmcp.ExtractStdoutFromResponse(output)
	var pvcsData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &pvcsData); err != nil {
		return "", "", fmt.Errorf("failed to parse PVCs: %w", err)
	}

	pvcs, ok := pvcsData["items"].([]interface{})
	if !ok || len(pvcs) == 0 {
		return "", "", fmt.Errorf("no PVCs found with labels plan=%s, migration=%s, vmID=%s", planID, migrationID, vmID)
	}

	// Get migration PVC UID
	var migrationPVCUID string
	for _, pvc := range pvcs {
		pvcMap, ok := pvc.(map[string]interface{})
		if !ok {
			continue
		}
		metadata, ok := pvcMap["metadata"].(map[string]interface{})
		if !ok {
			continue
		}
		uid, ok := metadata["uid"].(string)
		if ok && uid != "" {
			migrationPVCUID = uid
			break
		}
	}

	if migrationPVCUID == "" {
		return "", "", fmt.Errorf("could not find migration PVC UID")
	}

	// Find prime PVC owned by migration PVC
	args = []string{"get", "pvc", "-n", namespace, "-o", "json"}
	output, err = mtvmcp.RunKubectlCommand(ctx, args)
	if err != nil {
		return "", "", fmt.Errorf("failed to get all PVCs: %w", err)
	}

	stdout = mtvmcp.ExtractStdoutFromResponse(output)
	var allPVCsData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &allPVCsData); err != nil {
		return "", "", fmt.Errorf("failed to parse all PVCs: %w", err)
	}

	allPVCs, ok := allPVCsData["items"].([]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected PVC list format")
	}

	// Look for prime PVC with importer pod annotation
	for _, pvc := range allPVCs {
		pvcMap, ok := pvc.(map[string]interface{})
		if !ok {
			continue
		}
		metadata, ok := pvcMap["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check if owned by migration PVC
		owners, _ := metadata["ownerReferences"].([]interface{})
		for _, owner := range owners {
			ownerMap, ok := owner.(map[string]interface{})
			if !ok {
				continue
			}
			ownerUID, _ := ownerMap["uid"].(string)
			if ownerUID == migrationPVCUID {
				// Found prime PVC, get importer pod name from annotation
				annotations, _ := metadata["annotations"].(map[string]interface{})
				if podName, ok := annotations["cdi.kubevirt.io/storage.import.importPodName"].(string); ok && podName != "" {
					return podName, namespace, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("could not find importer pod name in PVC annotations")
}
