package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// GetPlanVmsInput represents the input for GetPlanVms
type GetPlanVmsInput struct {
	PlanName  string `json:"plan_name" jsonschema:"Name of the migration plan to query"`
	Namespace string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace containing the plan (optional)"`
}

// GetGetPlanVmsTool returns the tool definition
func GetGetPlanVmsTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "GetPlanVms",
		Description: `Get VMs and their status from a specific migration plan.

    This shows all VMs included in a migration plan along with their current migration status,
    progress, and any issues. Essential for monitoring migration progress and troubleshooting
    specific VM migration problems.

    Args:
        plan_name: Name of the migration plan to query
        namespace: Kubernetes namespace containing the plan (optional)

    Returns:
        JSON formatted VM status information

    Integration with Write Tools:
        Use this tool to monitor migration progress and troubleshoot:
        1. Monitor progress: get_plan_vms("my-plan")
        2. Cancel problematic VMs: cancel_plan("my-plan", "failed-vm1,stuck-vm2")
        3. Get detailed logs: get_logs("importer", plan_id="...", migration_id="...", vm_id="...")`,
	}
}

func HandleGetPlanVms(ctx context.Context, req *mcp.CallToolRequest, input GetPlanVmsInput) (*mcp.CallToolResult, any, error) {
	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"plan_name": input.PlanName,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"get", "plan", input.PlanName, "--vms"}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	args = append(args, "-o", "json")

	result, err := mtvmcp.RunKubectlMTVCommand(args)
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
