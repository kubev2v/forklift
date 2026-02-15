package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
)

// MTVHelpInput represents the input for the mtv_help tool.
type MTVHelpInput struct {
	// Command is the kubectl-mtv command or topic to get help for.
	// Examples: "create plan", "get inventory vm", "tsl", "karl"
	Command string `json:"command" jsonschema:"Command or topic (e.g. create plan, get inventory vm, tsl, karl)"`
}

// GetMTVHelpTool returns the tool definition for on-demand help.
func GetMTVHelpTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "mtv_help",
		Description: `Get help: flags, usage, examples for any command, or syntax refs for topics.

Commands: any from mtv_read/mtv_write, e.g.:
  "get plan", "get provider", "get inventory vm"
  "describe plan", "create provider", "create plan"
  "start plan", "patch plan", "delete plan"
Topics:
  "tsl" - VM query language with field list per provider (vSphere, oVirt, OpenStack, EC2)
  "karl" - VM placement affinity/anti-affinity rules (e.g. "REQUIRE tag:zone=us-east")

IMPORTANT: Call mtv_help("tsl") before writing inventory queries to learn available fields and syntax.

Output: Returns command flags, usage, and examples as structured data.`,
		OutputSchema: mtvOutputSchema,
	}
}

// HandleMTVHelp handles the mtv_help tool invocation.
func HandleMTVHelp(ctx context.Context, req *mcp.CallToolRequest, input MTVHelpInput) (*mcp.CallToolResult, any, error) {
	// Extract K8s credentials from HTTP headers (populated by wrapper in SSE mode)
	ctx = extractKubeCredsFromRequest(ctx, req)

	command := strings.TrimSpace(input.Command)
	if command == "" {
		return nil, nil, fmt.Errorf("command is required (e.g. \"create plan\", \"tsl\", \"karl\")")
	}

	// Build args: help --machine [command parts...]
	args := []string{"help", "--machine"}
	parts := strings.Fields(command)
	args = append(args, parts...)

	// Execute kubectl-mtv help --machine [command]
	result, err := util.RunKubectlMTVCommand(ctx, args)
	if err != nil {
		return nil, nil, fmt.Errorf("help command failed: %w", err)
	}

	// Parse and return result
	data, err := util.UnmarshalJSONResponse(result)
	if err != nil {
		return nil, nil, err
	}

	// Post-process: convert CLI-style help to MCP-style for LLM consumption.
	// This handles both single-command responses and multi-command (array) responses.
	convertHelpToMCPStyle(data)

	return nil, data, nil
}

// convertHelpToMCPStyle transforms CLI-style help data into MCP-style.
// It modifies the data map in place, converting:
// - "usage" field from CLI format to MCP call pattern
// - "examples" from CLI commands to MCP JSON-style calls
// Works for both single command (data.commands is a single object) and
// multi-command responses (data.commands is an array).
func convertHelpToMCPStyle(data map[string]interface{}) {
	// Handle the "data" envelope — help --machine returns {command, return_value, data}
	rawData, ok := data["data"]
	if !ok {
		return
	}

	switch d := rawData.(type) {
	case map[string]interface{}:
		// Single command or topic response — check if it has "commands" (array of commands)
		if commands, ok := d["commands"].([]interface{}); ok {
			for _, cmdRaw := range commands {
				if cmd, ok := cmdRaw.(map[string]interface{}); ok {
					convertCommandToMCPStyle(cmd)
				}
			}
		} else {
			// Single command response (e.g., help --machine get plan)
			convertCommandToMCPStyle(d)
		}
	case []interface{}:
		// Array of commands
		for _, cmdRaw := range d {
			if cmd, ok := cmdRaw.(map[string]interface{}); ok {
				convertCommandToMCPStyle(cmd)
			}
		}
	}
}

// convertCommandToMCPStyle transforms a single command's help data to MCP-style.
// It replaces the "usage" field with an MCP call pattern and converts CLI examples
// to MCP-style JSON calls.
func convertCommandToMCPStyle(cmd map[string]interface{}) {
	pathString, _ := cmd["path_string"].(string)
	if pathString == "" {
		return
	}

	// Build flag shorthand -> long name mapping
	shortToLong := make(map[string]string)
	if rawFlags, ok := cmd["flags"].([]interface{}); ok {
		for _, rawFlag := range rawFlags {
			if flag, ok := rawFlag.(map[string]interface{}); ok {
				name, _ := flag["name"].(string)
				shorthand, _ := flag["shorthand"].(string)
				if shorthand != "" && name != "" {
					shortToLong[shorthand] = name
				}
			}
		}
	}

	// Replace "usage" with MCP-style call pattern
	cmd["usage"] = fmt.Sprintf("{command: \"%s\", flags: {...}}", pathString)

	// Convert examples from CLI to MCP style
	if rawExamples, ok := cmd["examples"].([]interface{}); ok {
		for i, rawEx := range rawExamples {
			ex, ok := rawEx.(map[string]interface{})
			if !ok {
				continue
			}
			cliCmd, _ := ex["command"].(string)
			if cliCmd == "" {
				continue
			}
			mcpCall := convertCLIExampleToMCP(cliCmd, pathString, shortToLong)
			if mcpCall != "" {
				ex["command"] = mcpCall
				rawExamples[i] = ex
			}
		}
	}
}

// cliContinuationRegex matches backslash-newline-comment patterns from multi-line CLI examples.
// Cobra examples use "\" followed by "# " comment lines for readability.
var cliContinuationRegex = regexp.MustCompile(`\s*\\\s*(?:#[^\n]*)?\n\s*(?:#\s*)?`)

// convertCLIExampleToMCP converts a CLI example string to an MCP-style JSON call.
// Example: "kubectl-mtv get inventory vm --provider vsphere-prod --query \"where name ~= 'web-.*'\""
// becomes: {command: "get inventory vm", flags: {provider: "vsphere-prod", query: "where name ~= 'web-.*'"}}
func convertCLIExampleToMCP(cliCmd string, pathString string, shortToLong map[string]string) string {
	// Clean up multi-line examples (backslash continuations with # comments)
	cliCmd = cliContinuationRegex.ReplaceAllString(cliCmd, " ")
	cliCmd = strings.TrimSpace(cliCmd)

	// Strip "kubectl-mtv " or "kubectl mtv " prefix
	cliCmd = strings.TrimPrefix(cliCmd, "kubectl-mtv ")
	cliCmd = strings.TrimPrefix(cliCmd, "kubectl mtv ")

	// Strip the command path from the front
	rest := cliCmd
	pathParts := strings.Fields(pathString)
	for _, part := range pathParts {
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, part+" ") {
			rest = rest[len(part)+1:]
		} else if rest == part {
			rest = ""
		}
	}
	rest = strings.TrimSpace(rest)

	// Tokenize the remaining string, respecting quotes
	tokens := tokenizeCLIArgs(rest)

	// Parse tokens as flags (no positional args — all args are flags now)
	flags := make(map[string]string)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !strings.HasPrefix(tok, "-") {
			continue // skip stray tokens
		}

		// Handle --flag=value
		if strings.Contains(tok, "=") {
			parts := strings.SplitN(tok, "=", 2)
			name := strings.TrimLeft(parts[0], "-")
			// Resolve shorthand
			if long, ok := shortToLong[name]; ok {
				name = long
			}
			flags[name] = parts[1]
			continue
		}

		// Handle --flag value or -f value
		name := strings.TrimLeft(tok, "-")
		// Resolve shorthand
		if long, ok := shortToLong[name]; ok {
			name = long
		}

		// Check if next token is a value (not another flag)
		if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
			flags[name] = tokens[i+1]
			i++ // skip value
		} else {
			// Boolean flag
			flags[name] = "true"
		}
	}

	// Build MCP-style call string
	if len(flags) == 0 {
		return fmt.Sprintf("{command: \"%s\"}", pathString)
	}

	var flagParts []string
	for k, v := range flags {
		if v == "true" {
			flagParts = append(flagParts, fmt.Sprintf("%s: true", k))
		} else {
			flagParts = append(flagParts, fmt.Sprintf("%s: \"%s\"", k, v))
		}
	}
	return fmt.Sprintf("{command: \"%s\", flags: {%s}}", pathString, strings.Join(flagParts, ", "))
}

// tokenizeCLIArgs splits a CLI argument string into tokens, respecting single and double quotes.
// Examples:
//
//	"hello world" -> ["hello", "world"]
//	"--name 'my plan'" -> ["--name", "my plan"]
//	"--query \"where name ~= 'test'\"" -> ["--query", "where name ~= 'test'"]
func tokenizeCLIArgs(s string) []string {
	var tokens []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote
		case ch == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote
		case ch == ' ' && !inSingleQuote && !inDoubleQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
