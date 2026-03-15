package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
)

// extractKubeCredsFromRequest extracts Kubernetes credentials from the request's
// Extra.Header field and adds them to the context. The wrapper in mcpserver.go
// ensures that Extra.Header is populated for SSE mode.
func extractKubeCredsFromRequest(ctx context.Context, req *mcp.CallToolRequest) context.Context {
	if req.Extra != nil && req.Extra.Header != nil {
		return util.WithKubeCredsFromHeaders(ctx, req.Extra.Header)
	}
	return ctx
}

// AddToolWithCoercion registers a tool with the server using the low-level
// s.AddTool API, adding a boolean coercion layer that converts string boolean
// values ("true", "True", "TRUE", "false", "False", "FALSE") to actual JSON
// booleans before unmarshaling into the typed input struct.
//
// This is needed because some AI models send boolean parameters as strings
// (e.g., Python-style "True" instead of JSON true), which the MCP SDK's
// strict schema validation rejects.
//
// The function:
//  1. Generates the input schema from the In type using jsonschema.For[In]
//  2. Creates a raw ToolHandler that coerces string booleans before unmarshal
//  3. Registers via s.AddTool (bypassing the SDK's strict validation)
func AddToolWithCoercion[In, Out any](s *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	// Generate input schema from the In type if not already set
	if t.InputSchema == nil {
		schema, err := jsonschema.For[In](nil)
		if err != nil {
			panic(fmt.Sprintf("AddToolWithCoercion: tool %q: failed to generate input schema: %v", t.Name, err))
		}
		t.InputSchema = schema
	}

	rawHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input json.RawMessage
		if req.Params.Arguments != nil {
			input = req.Params.Arguments
		}

		// Coerce string booleans to actual booleans based on the In struct's bool fields
		input = CoerceBooleans[In](input)

		// Unmarshal into typed input
		var in In
		if input != nil {
			if err := json.Unmarshal(input, &in); err != nil {
				return nil, fmt.Errorf("invalid params: %v", err)
			}
		}

		// Call the typed handler
		res, out, err := h(ctx, req, in)

		// Handle errors: wrap as tool errors (IsError=true), not protocol errors.
		// This matches the SDK's ToolHandlerFor behavior (server.go lines 283-291).
		if err != nil {
			var errRes mcp.CallToolResult
			errRes.Content = []mcp.Content{&mcp.TextContent{Text: err.Error()}}
			errRes.IsError = true
			return &errRes, nil
		}

		if res == nil {
			res = &mcp.CallToolResult{}
		}

		// Marshal the output and populate StructuredContent + Content.
		// This replicates the SDK's output serialization (server.go lines 298-332).
		var outval any = out
		if outval != nil {
			outBytes, err := json.Marshal(outval)
			if err != nil {
				return nil, fmt.Errorf("marshaling output: %w", err)
			}
			outJSON := json.RawMessage(outBytes)
			res.StructuredContent = outJSON

			if res.Content == nil {
				res.Content = []mcp.Content{&mcp.TextContent{
					Text: string(outJSON),
				}}
			}
		}

		return res, nil
	}

	s.AddTool(t, rawHandler)
}

// CoerceBooleans examines the In type's struct fields via reflection, finds
// all bool fields, and coerces any corresponding string values in the JSON
// data to actual JSON booleans. This allows clients that send "True"/"true"
// as strings to work correctly.
//
// If the data is not valid JSON or the In type is not a struct, the original
// data is returned unchanged.
func CoerceBooleans[In any](data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return data
	}

	rt := reflect.TypeFor[In]()
	// Follow pointers to the underlying type
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return data
	}

	changed := false
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}

		// Extract the JSON key from the struct tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		jsonKey := strings.Split(jsonTag, ",")[0]
		if jsonKey == "" {
			continue
		}

		// Check if the value is a string that should be coerced to bool
		if v, ok := m[jsonKey]; ok {
			if s, ok := v.(string); ok {
				switch strings.ToLower(s) {
				case "true":
					m[jsonKey] = true
					changed = true
				case "false":
					m[jsonKey] = false
					changed = true
				}
			}
		}
	}

	if !changed {
		return data
	}

	result, err := json.Marshal(m)
	if err != nil {
		return data
	}
	return result
}
