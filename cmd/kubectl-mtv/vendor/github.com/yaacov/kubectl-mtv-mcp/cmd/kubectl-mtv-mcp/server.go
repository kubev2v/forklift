package cmd

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/cmd/kubectl-mtv-mcp/tools"
)

func CreateReadServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubectl-mtv",
		Version: Version,
	}, nil)

	// Register read-only tools
	mcp.AddTool(server, tools.GetListResourcesTool(), tools.HandleListResources)
	mcp.AddTool(server, tools.GetListInventoryTool(), tools.HandleListInventory)
	mcp.AddTool(server, tools.GetQueryPodsTool(), tools.HandleQueryPods)
	mcp.AddTool(server, tools.GetQueryLogsTool(), tools.HandleQueryLogs)
	mcp.AddTool(server, tools.GetGetMigrationStorageTool(), tools.HandleGetMigrationStorage)
	mcp.AddTool(server, tools.GetGetPlanVmsTool(), tools.HandleGetPlanVms)

	// GetVersion tool - kept here since extraction script skipped it
	mcp.AddTool(server, &mcp.Tool{
		Name: "GetVersion",
		Description: `Get kubectl-mtv and MTV operator version information.

This tool provides comprehensive version information including:
- kubectl-mtv client version
- MTV operator version and status
- MTV operator namespace
- MTV inventory service URL and availability

This is essential for troubleshooting MTV setup and understanding the deployment.

Returns:
    Version information in JSON format`,
	}, handleGetVersion)

	// Register write tools (USE WITH CAUTION)
	mcp.AddTool(server, tools.GetManagePlanLifecycleTool(), tools.HandleManagePlanLifecycle)
	mcp.AddTool(server, tools.GetCreateProviderTool(), tools.HandleCreateProvider)
	mcp.AddTool(server, tools.GetManageMappingTool(), tools.HandleManageMapping)
	mcp.AddTool(server, tools.GetCreatePlanTool(), tools.HandleCreatePlan)
	mcp.AddTool(server, tools.GetCreateHostTool(), tools.HandleCreateHost)
	mcp.AddTool(server, tools.GetCreateHookTool(), tools.HandleCreateHook)
	mcp.AddTool(server, tools.GetDeleteProviderTool(), tools.HandleDeleteProvider)
	mcp.AddTool(server, tools.GetDeletePlanTool(), tools.HandleDeletePlan)
	mcp.AddTool(server, tools.GetDeleteHostTool(), tools.HandleDeleteHost)
	mcp.AddTool(server, tools.GetDeleteHookTool(), tools.HandleDeleteHook)
	mcp.AddTool(server, tools.GetPatchProviderTool(), tools.HandlePatchProvider)
	mcp.AddTool(server, tools.GetPatchPlanTool(), tools.HandlePatchPlan)
	mcp.AddTool(server, tools.GetPatchPlanVmTool(), tools.HandlePatchPlanVm)

	return server
}
