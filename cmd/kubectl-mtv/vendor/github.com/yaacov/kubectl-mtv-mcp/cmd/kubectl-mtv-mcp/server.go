package cmd

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp/discovery"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp/tools"
)

// CreateServer creates the MCP server with dynamically discovered tools.
// Discovery happens at startup using kubectl-mtv help --machine.
func CreateServer() (*mcp.Server, error) {
	ctx := context.Background()
	registry, err := discovery.NewRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover commands: %w", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubectl-mtv",
		Version: Version,
	}, nil)

	// Register the three dynamic tools
	mcp.AddTool(server, tools.GetMTVReadTool(registry), tools.HandleMTVRead(registry))
	mcp.AddTool(server, tools.GetMTVWriteTool(registry), tools.HandleMTVWrite(registry))
	mcp.AddTool(server, tools.GetKubectlDebugTool(), tools.HandleKubectlDebug)

	return server, nil
}
