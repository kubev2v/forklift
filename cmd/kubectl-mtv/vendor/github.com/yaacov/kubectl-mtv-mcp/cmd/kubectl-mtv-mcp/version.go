package cmd

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

func handleGetVersion(ctx context.Context, req *mcp.CallToolRequest, input struct {
	RandomString string `json:"random_string"`
}) (*mcp.CallToolResult, any, error) {
	args := []string{"version", "-o", "json"}
	result, err := mtvmcp.RunKubectlMTVCommand(ctx, args)
	if err != nil {
		return nil, "", err
	}
	// Unmarshal the JSON string into a native object for the MCP SDK
	data, err := mtvmcp.UnmarshalJSONResponse(result)
	if err != nil {
		return nil, "", err
	}
	return nil, data, nil
}
