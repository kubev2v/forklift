package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// ManagePlanLifecycleInput represents the input for ManagePlanLifecycle
type ManagePlanLifecycleInput struct {
	Action    string `json:"action" jsonschema:"Lifecycle action - 'start', 'cancel', 'cutover', 'archive', or 'unarchive'"`
	PlanName  string `json:"plan_name" jsonschema:"Name of the migration plan (supports space-separated names for start action)"`
	Namespace string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace containing the plan (optional)"`
	Cutover   string `json:"cutover,omitempty" jsonschema:"Cutover time in ISO8601 format for start action (optional)"`
	VMs       string `json:"vms,omitempty" jsonschema:"VM names for cancel action - comma-separated or @filename (required for cancel)"`
	DryRun    bool   `json:"dry_run,omitempty" jsonschema:"If true, shows commands instead of executing (educational mode)"`
}

// GetManagePlanLifecycleTool returns the tool definition
func GetManagePlanLifecycleTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "ManagePlanLifecycle",
		Description: `Manage migration plan lifecycle operations.

    Unified tool for all plan lifecycle actions including start, cancel, cutover, archive, and unarchive.
    Each action has specific prerequisites and effects on the migration process.

    Dry Run Mode: Set dry_run=true to see the command without executing (useful for teaching users)

    Actions:
    - 'start': Begin migrating VMs in the plan
    - 'cancel': Cancel specific VMs in a running migration
    - 'cutover': Perform final cutover phase
    - 'archive': Archive completed migration plan
    - 'unarchive': Restore archived migration plan

    Action-Specific Parameters:
    - start: cutover (optional ISO8601 timestamp for warm migrations)
    - cancel: vms (required - VM names to cancel)
    - cutover, archive, unarchive: no additional parameters

    Prerequisites by Action:
    Start:
    - Provider connectivity must be validated
    - Network and storage mappings must be configured
    - VM inventory must be current and accessible
    - Target namespace must exist (if specified)

    Cancel:
    - Plan must be actively running
    - VMs must not have completed migration

    Cutover:
    - Plan must be in warm migration state
    - Initial data sync must be complete

    Archive/Unarchive:
    - Plan must be in completed/archived state respectively

    Args:
        action: Lifecycle action - 'start', 'cancel', 'cutover', 'archive', 'unarchive'
        plan_name: Name of the migration plan (supports space-separated names for start action)
        namespace: Kubernetes namespace containing the plan (optional)
        cutover: Cutover time in ISO8601 format for start action (optional)
        vms: VM names for cancel action - comma-separated or @filename (required for cancel)

    Returns:
        Command output confirming the lifecycle action

    Examples:
        # Start plan immediately
        ManagePlanLifecycle(action="start", plan_name="production-migration")

        # Start plan with scheduled cutover
        ManagePlanLifecycle(action="start", plan_name="production-migration", cutover="2023-12-25T02:00:00Z")

        # Start multiple plans
        ManagePlanLifecycle(action="start", plan_name="plan1 plan2 plan3")

        # Cancel specific VMs
        ManagePlanLifecycle(action="cancel", plan_name="production-migration", vms="webserver-01,database-02")

        # Cancel VMs from file
        ManagePlanLifecycle(action="cancel", plan_name="production-migration", vms="@vms-to-cancel.json")

        # Perform cutover
        ManagePlanLifecycle(action="cutover", plan_name="production-migration")

        # Archive plan
        ManagePlanLifecycle(action="archive", plan_name="production-migration")

        # Unarchive plan
        ManagePlanLifecycle(action="unarchive", plan_name="production-migration")`,
	}
}

// HandleManagePlanLifecycle handles plan lifecycle operations
func HandleManagePlanLifecycle(ctx context.Context, req *mcp.CallToolRequest, input ManagePlanLifecycleInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"action":    input.Action,
		"plan_name": input.PlanName,
	}); err != nil {
		return nil, "", err
	}

	// Validate action
	validActions := []string{"start", "cancel", "cutover", "archive", "unarchive"}
	found := false
	for _, a := range validActions {
		if input.Action == a {
			found = true
			break
		}
	}
	if !found {
		return nil, "", fmt.Errorf("invalid action '%s'. Valid actions: %v", input.Action, validActions)
	}

	// Validate action-specific requirements
	if input.Action == "cancel" && input.VMs == "" {
		return nil, "", fmt.Errorf("the 'vms' parameter is required for cancel action")
	}

	var args []string

	switch input.Action {
	case "start":
		planNames := strings.Fields(input.PlanName)
		args = append([]string{"start", "plan"}, planNames...)
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}
		if input.Cutover != "" {
			args = append(args, "--cutover", input.Cutover)
		}

	case "cancel":
		args = []string{"cancel", "plan", input.PlanName, "--vms", input.VMs}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}

	case "cutover":
		args = []string{"cutover", "plan", input.PlanName}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}

	case "archive":
		args = []string{"archive", "plan", input.PlanName}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}

	case "unarchive":
		args = []string{"unarchive", "plan", input.PlanName}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}
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
