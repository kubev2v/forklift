package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetLogsInput represents the input for GetLogs
type GetLogsInput struct {
	PodType     string `json:"pod_type,omitempty" jsonschema:"Type of pod to get logs from ('controller' or 'importer'). Defaults to 'controller'"`
	Container   string `json:"container,omitempty" jsonschema:"Container name for controller pods (main, inventory). Defaults to 'main'"`
	Lines       int    `json:"lines,omitempty" jsonschema:"Number of recent log lines to retrieve. Defaults to 100"`
	Follow      bool   `json:"follow,omitempty" jsonschema:"Follow log output (stream logs). Not recommended for MCP usage"`
	Namespace   string `json:"namespace,omitempty" jsonschema:"Override namespace (optional, auto-detected for controller)"`
	PlanID      string `json:"plan_id,omitempty" jsonschema:"Plan UUID for finding importer pods (required for importer type)"`
	MigrationID string `json:"migration_id,omitempty" jsonschema:"Migration UUID for finding importer pods (required for importer type)"`
	VMID        string `json:"vm_id,omitempty" jsonschema:"VM ID for finding importer pods (required for importer type)"`
}

// GetGetLogsTool returns the tool definition
func GetGetLogsTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "GetLogs",
		Description: `Get logs from MTV-related pods for debugging.

    This tool can retrieve logs from:
    1. MTV controller pod (main, inventory containers) - auto-detects namespace and pod
    2. Importer pods - finds pod using migration labels and prime PVC annotations

    Pod Types:
    - controller: MTV forklift-controller pod (default)
    - importer: CDI importer pod for VM disk migration

    For controller pods:
    - Automatically finds MTV operator namespace and running controller pod
    - Supports 'main' and 'inventory' containers

    For importer pods:
    - Uses plan_id, migration_id, vm_id to find migration PVCs
    - Locates prime PVC with cdi.kubevirt.io/storage.import.importPodName annotation
    - Retrieves logs from the importer pod

    Args:
        pod_type: Type of pod to get logs from ('controller' or 'importer'). Defaults to 'controller'
        container: Container name for controller pods (main, inventory). Defaults to 'main'
        lines: Number of recent log lines to retrieve. Defaults to 100
        follow: Follow log output (stream logs). Not recommended for MCP usage
        namespace: Override namespace (optional, auto-detected for controller)
        plan_id: Plan UUID for finding importer pods (required for importer type)
        migration_id: Migration UUID for finding importer pods (required for importer type)
        vm_id: VM ID for finding importer pods (required for importer type)

    Returns:
        JSON structure containing pod information and logs:
        {
            "pod": { ... pod JSON with status, conditions, etc ... },
            "logs": "pod logs content"
        }

    Examples:
        # Get controller main container logs
        get_logs("controller", "main", 200)

        # Get controller inventory container logs
        get_logs("controller", "inventory", 100)

        # Get importer pod logs for specific VM migration
        get_logs("importer", "", 100, False, "demo", "plan-uuid", "migration-uuid", "vm-47")`,
	}
}

func HandleGetLogs(ctx context.Context, req *mcp.CallToolRequest, input GetLogsInput) (*mcp.CallToolResult, any, error) {
	podType := input.PodType
	if podType == "" {
		podType = "controller"
	}

	container := input.Container
	if container == "" {
		container = "main"
	}

	lines := input.Lines
	if lines == 0 {
		lines = 100
	}

	if podType == "controller" {
		return getControllerLogs(ctx, container, lines, input.Follow, input.Namespace)
	} else if podType == "importer" {
		if input.PlanID == "" || input.MigrationID == "" || input.VMID == "" {
			return nil, "", fmt.Errorf("for importer logs, plan_id, migration_id, and vm_id are required")
		}
		return getImporterLogs(ctx, lines, input.Follow, input.Namespace, input.PlanID, input.MigrationID, input.VMID)
	}

	return nil, "", fmt.Errorf("unknown pod_type '%s'. Supported types: 'controller', 'importer'", podType)
}
