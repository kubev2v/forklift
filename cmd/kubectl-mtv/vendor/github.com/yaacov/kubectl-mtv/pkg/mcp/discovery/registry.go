package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Registry holds discovered kubectl-mtv commands organized by read/write access.
type Registry struct {
	// ReadOnly contains commands that don't modify cluster state
	ReadOnly map[string]*Command

	// ReadOnlyOrder preserves the Cobra registration order of read-only
	// commands from help --machine. Used for all iteration: command listings,
	// example selection, and descriptions.
	ReadOnlyOrder []string

	// ReadWrite contains commands that modify cluster state
	ReadWrite map[string]*Command

	// ReadWriteOrder preserves the Cobra registration order of read-write commands.
	ReadWriteOrder []string

	// Parents contains non-runnable structural parent commands
	// (e.g., "get/inventory") for description lookup during group compaction.
	Parents map[string]*Command

	// GlobalFlags are flags available to all commands
	GlobalFlags []Flag

	// RootDescription is the main kubectl-mtv short description
	RootDescription string

	// LongDescription is the extended CLI description with domain context
	// (e.g., "Migrate virtual machines from VMware vSphere, oVirt...")
	LongDescription string
}

// NewRegistry creates a new registry by calling kubectl-mtv help --machine.
// This single call returns the complete command schema as JSON.
// It uses os.Executable() to call the same binary that is running the MCP server,
// ensuring the help schema always matches the server's code (avoids version mismatch
// when a different kubectl-mtv version is installed in PATH).
func NewRegistry(ctx context.Context) (*Registry, error) {
	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Use the current executable to ensure help matches the running server
	self, err := os.Executable()
	if err != nil {
		// Fall back to PATH lookup if os.Executable fails
		self = "kubectl-mtv"
	}
	cmd := exec.CommandContext(cmdCtx, self, "help", "--machine")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine help: %w", err)
	}

	var schema HelpSchema
	if err := json.Unmarshal(output, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse help schema: %w", err)
	}

	registry := &Registry{
		ReadOnly:        make(map[string]*Command),
		ReadWrite:       make(map[string]*Command),
		Parents:         make(map[string]*Command),
		GlobalFlags:     schema.GlobalFlags,
		RootDescription: schema.Description,
		LongDescription: schema.LongDescription,
	}

	// Categorize commands by read/write based on category field.
	// Admin commands (completions, help, mcp-server, version) are skipped
	// entirely — they are irrelevant to LLM tool use.
	// Non-runnable parent commands are stored separately for description lookup.
	//
	// Backward compatibility: older CLI versions may not emit the "runnable" field.
	// We detect this by checking if ANY command has Runnable=true. If none do,
	// the schema predates the field and all commands are leaf commands.
	schemaHasRunnable := false
	for i := range schema.Commands {
		if schema.Commands[i].Runnable {
			schemaHasRunnable = true
			break
		}
	}

	for i := range schema.Commands {
		cmd := &schema.Commands[i]
		pathKey := cmd.PathKey()

		// Determine if this command is runnable:
		// - If the schema has the field: trust cmd.Runnable
		// - If the schema lacks the field: all commands are leaf (runnable)
		isRunnable := cmd.Runnable || !schemaHasRunnable

		// Store non-runnable parents separately for group description lookup
		if !isRunnable {
			registry.Parents[pathKey] = cmd
			continue
		}

		switch cmd.Category {
		case "read":
			registry.ReadOnly[pathKey] = cmd
			registry.ReadOnlyOrder = append(registry.ReadOnlyOrder, pathKey)
		case "admin":
			// Skip admin commands (shell completions, help, version, etc.)
			continue
		default:
			// "write" category
			registry.ReadWrite[pathKey] = cmd
			registry.ReadWriteOrder = append(registry.ReadWriteOrder, pathKey)
		}
	}

	return registry, nil
}

// ListReadOnlyCommands returns read-only command paths in Cobra registration order.
func (r *Registry) ListReadOnlyCommands() []string {
	out := make([]string, len(r.ReadOnlyOrder))
	copy(out, r.ReadOnlyOrder)
	return out
}

// ListReadWriteCommands returns read-write command paths in Cobra registration order.
func (r *Registry) ListReadWriteCommands() []string {
	out := make([]string, len(r.ReadWriteOrder))
	copy(out, r.ReadWriteOrder)
	return out
}

// IsReadOnly checks if a command path is read-only.
func (r *Registry) IsReadOnly(pathKey string) bool {
	_, ok := r.ReadOnly[pathKey]
	return ok
}

// IsReadWrite checks if a command path is read-write.
func (r *Registry) IsReadWrite(pathKey string) bool {
	_, ok := r.ReadWrite[pathKey]
	return ok
}

// GenerateServerInstructions generates the MCP server-level instructions sent
// during initialization. This gives the LLM domain context (what MTV/Forklift is)
// and establishes the tool usage workflow before it sees any tool descriptions.
func (r *Registry) GenerateServerInstructions() string {
	var sb strings.Builder

	sb.WriteString("MTV (Migration Toolkit for Virtualization), also known as Forklift, migrates virtual machines from VMware vSphere, oVirt (RHV), OpenStack, and Amazon EC2 into OpenShift Virtualization (KubeVirt).\n")
	sb.WriteString("\nThis server provides three tools:\n")
	sb.WriteString("  mtv_read  - Query resources (plans, providers, inventory, mappings, health, logs, settings)\n")
	sb.WriteString("  mtv_write - Create, modify, or delete resources (providers, plans, mappings, hooks)\n")
	sb.WriteString("  mtv_help  - Get detailed flags, syntax, and examples for any command\n")
	sb.WriteString("\nWorkflow:\n")
	sb.WriteString("  1. Find the command you need in mtv_read or mtv_write\n")
	sb.WriteString("  2. Call mtv_help(\"<command>\") to learn its flags and see examples\n")
	sb.WriteString("  3. Execute the command with the correct flags\n")
	sb.WriteString("\nThe tool descriptions list available commands but not their flags — always call mtv_help first for unfamiliar commands.\n")

	return sb.String()
}

// GenerateReadOnlyDescription generates the description for the read-only tool.
// It puts the command list first (most critical for the LM), followed by examples
// and a hint to use mtv_help.
func (r *Registry) GenerateReadOnlyDescription() string {
	var sb strings.Builder

	sb.WriteString("Query MTV resources (read-only).\n")
	sb.WriteString("\nCommands:\n")

	// Detect deep sibling groups (depth >= 3) for compaction
	groups, groupedKeys := detectDeepSiblingGroups(r.ReadOnly, r.Parents)

	// List non-grouped commands normally
	commands := r.ListReadOnlyCommands()
	for _, key := range commands {
		if groupedKeys[key] {
			continue
		}
		cmd := r.ReadOnly[key]
		sb.WriteString(fmt.Sprintf("  %s - %s\n", cmd.CommandPath(), cmd.Description))
	}

	// Write compacted sibling groups
	for _, group := range groups {
		parentDisplay := strings.ReplaceAll(group.parentPath, "/", " ")
		sb.WriteString(fmt.Sprintf("  %s RESOURCE - %s\n", parentDisplay, group.description))
		sb.WriteString(fmt.Sprintf("    Resources: %s\n", strings.Join(group.children, ", ")))
	}

	// Examples: first example from each command in order, capped at N
	examples := r.collectOrderedExamples(r.ReadOnly, r.ReadOnlyOrder, 10)
	if len(examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, ex := range examples {
			sb.WriteString(fmt.Sprintf("  %s\n", ex))
		}
	}

	sb.WriteString(r.formatGlobalFlags())

	sb.WriteString("\nWORKFLOW: Find the command above → call mtv_help(\"<command>\") to get its flags and examples → then execute.\n")
	sb.WriteString("Default output is table. For structured data, use flags: {output: \"json\"} and fields: [\"name\", \"status\"] to limit response size.\n")
	sb.WriteString("IMPORTANT: 'fields' is a TOP-LEVEL parameter, NOT inside flags. Example: {command: \"get plan\", flags: {output: \"json\"}, fields: [\"name\", \"status\"]}\n")
	sb.WriteString("RULE: ALWAYS include namespace or all_namespaces in flags. Never omit both.\n")
	sb.WriteString("RULE: Only include optional flags (e.g. mapping pairs, skip-verify) when the user explicitly needs them. Leave them empty otherwise.\n")
	return sb.String()
}

// GenerateReadWriteDescription generates the description for the read-write tool.
// It puts the command list first (most critical for the LM), followed by examples
// and a hint to use mtv_help.
func (r *Registry) GenerateReadWriteDescription() string {
	// Detect bare parent commands to skip them from the listing.
	// Admin commands are already filtered out by NewRegistry.
	bareParents := detectBareParents(r.ReadWrite)

	var sb strings.Builder

	sb.WriteString("Create, modify, or delete MTV resources (write operations).\n")
	sb.WriteString("\nTypical migration workflow:\n")
	sb.WriteString("1. Set namespace for the migration\n")
	sb.WriteString("2. Check existing providers with mtv_read \"get provider\"; create via mtv_write \"create provider\" only if needed\n")
	sb.WriteString("3. Browse VMs with mtv_read \"get inventory vm\" + TSL queries\n")
	sb.WriteString("4. Create a migration plan (network/storage mappings are auto-generated; use --network-pairs/--storage-pairs to override)\n")
	sb.WriteString("5. Start the plan\n")
	sb.WriteString("6. Monitor with mtv_read \"get plan\"\n")
	sb.WriteString("\nCommands:\n")

	commands := r.ListReadWriteCommands()
	for _, key := range commands {
		if bareParents[key] {
			continue
		}
		cmd := r.ReadWrite[key]
		sb.WriteString(fmt.Sprintf("  %s - %s\n", cmd.CommandPath(), cmd.Description))
	}

	examples := r.collectOrderedExamples(r.ReadWrite, r.ReadWriteOrder, 10)
	if len(examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, ex := range examples {
			sb.WriteString(fmt.Sprintf("  %s\n", ex))
		}
	}

	sb.WriteString(r.formatGlobalFlags())

	sb.WriteString("\nWORKFLOW: Find the command above → call mtv_help(\"<command>\") to get required flags and examples → then execute.\n")
	sb.WriteString("RULE: ALWAYS include namespace or all_namespaces in flags. Never omit both.\n")
	sb.WriteString("RULE: Only include optional flags (e.g. mapping pairs, skip-verify) when the user explicitly needs them. Leave them empty otherwise.\n")
	return sb.String()
}

// collectOrderedExamples collects MCP-style examples by iterating commands in
// their help registration order and taking the first example from each command.
// No heuristics — the help output order is the source of truth.
func (r *Registry) collectOrderedExamples(commands map[string]*Command, orderedKeys []string, maxExamples int) []string {
	var examples []string
	for _, key := range orderedKeys {
		if len(examples) >= maxExamples {
			break
		}
		cmd := commands[key]
		if cmd == nil || len(cmd.Examples) == 0 {
			continue
		}
		mcpExamples := convertCLIToMCPExamples(cmd, 1)
		examples = append(examples, mcpExamples...)
	}
	return examples
}

// sensitiveFlags lists flag names whose values should not appear in MCP examples.
// These are replaced with a placeholder to avoid leaking credentials.
var sensitiveFlags = map[string]bool{
	"password": true, "token": true, "provider-token": true, "secret": true, "secret-name": true,
}

// convertCLIToMCPExamples converts up to n CLI examples of a command into
// MCP-style call format strings. CLI commands should define their most
// instructive examples first — this is the source of truth for MCP descriptions.
// Duplicate MCP call strings (e.g., examples that differ only in description)
// are deduplicated.
func convertCLIToMCPExamples(cmd *Command, n int) []string {
	if len(cmd.Examples) == 0 || n <= 0 {
		return nil
	}

	pathString := cmd.CommandPath()
	seen := make(map[string]bool)
	var results []string

	for _, ex := range cmd.Examples {
		if len(results) >= n {
			break
		}
		mcpCall := formatCLIAsMCP(ex.Command, pathString)
		if mcpCall != "" && !seen[mcpCall] {
			results = append(results, mcpCall)
			seen[mcpCall] = true
		}
	}
	return results
}

// formatCLIAsMCP parses a single CLI command string and converts it to
// MCP-style call format: {command: "...", flags: {...}}.
// It strips the CLI prefix and collects --flag value pairs.
//
// Examples:
//
//	"kubectl-mtv get plan" → {command: "get plan"}
//	"kubectl-mtv get inventory vm --provider vsphere-prod --query \"where len(nics) >= 2\"" →
//	  {command: "get inventory vm", flags: {provider: "vsphere-prod", query: "where len(nics) >= 2"}}
func formatCLIAsMCP(cliCmd string, pathString string) string {
	cliCmd = strings.TrimSpace(cliCmd)
	if cliCmd == "" {
		return ""
	}
	cliCmd = strings.TrimPrefix(cliCmd, "kubectl-mtv ")
	cliCmd = strings.TrimPrefix(cliCmd, "kubectl mtv ")

	// Strip the command path from the front to get flags
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

	if rest == "" {
		return fmt.Sprintf("{command: \"%s\"}", pathString)
	}

	tokens := shellTokenize(rest)
	flagMap := make(map[string]string)

	// Extract --flag value pairs (no positional args to handle)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !strings.HasPrefix(tok, "-") {
			continue // skip stray tokens
		}
		// Strip leading dashes and handle --flag=value syntax
		flagName := strings.TrimLeft(tok, "-")
		var flagValue string
		if eqIdx := strings.Index(flagName, "="); eqIdx >= 0 {
			flagValue = flagName[eqIdx+1:]
			flagName = flagName[:eqIdx]
		} else if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
			i++
			flagValue = tokens[i]
		} else {
			// Boolean flag (no value)
			flagValue = "true"
		}

		// Skip sensitive flags
		if sensitiveFlags[flagName] {
			continue
		}

		// Strip surrounding quotes
		flagValue = strings.Trim(flagValue, "'\"")

		// Use underscores for MCP JSON convention
		displayName := strings.ReplaceAll(flagName, "-", "_")
		flagMap[displayName] = flagValue
	}

	if len(flagMap) == 0 {
		return fmt.Sprintf("{command: \"%s\"}", pathString)
	}

	var flagParts []string
	for k, v := range flagMap {
		if v == "true" {
			flagParts = append(flagParts, fmt.Sprintf("%s: true", k))
		} else {
			flagParts = append(flagParts, fmt.Sprintf("%s: \"%s\"", k, v))
		}
	}
	// Sort for deterministic output
	sort.Strings(flagParts)
	return fmt.Sprintf("{command: \"%s\", flags: {%s}}", pathString, strings.Join(flagParts, ", "))
}

// shellTokenize splits a string into tokens, respecting double-quoted and
// single-quoted substrings so that values like "VM Network:default" remain
// a single token. Quotes are stripped from the returned tokens.
func shellTokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inDouble := false
	inSingle := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == ' ' && !inDouble && !inSingle:
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

// FormatCommandHelp formats a command's flags and examples as LLM-friendly help text
// suitable for embedding in error messages. Required flags are listed first with a
// (REQUIRED) marker, enum values are shown in brackets, and hidden flags are excluded.
// All examples are included, converted to MCP-style call format.
func FormatCommandHelp(cmd *Command) string {
	if cmd == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- Help for \"%s\" ---\n", cmd.CommandPath()))

	var required, optional []Flag
	for _, f := range cmd.Flags {
		if f.Hidden {
			continue
		}
		if f.Required {
			required = append(required, f)
		} else {
			optional = append(optional, f)
		}
	}

	if len(required) > 0 || len(optional) > 0 {
		sb.WriteString("Flags:\n")
		for _, f := range required {
			sb.WriteString(formatFlagLine(f, true))
		}
		for _, f := range optional {
			sb.WriteString(formatFlagLine(f, false))
		}
	}

	mcpExamples := convertCLIToMCPExamples(cmd, len(cmd.Examples))
	if len(mcpExamples) > 0 {
		if len(mcpExamples) == 1 {
			sb.WriteString(fmt.Sprintf("Example: %s\n", mcpExamples[0]))
		} else {
			sb.WriteString("Examples:\n")
			for _, ex := range mcpExamples {
				sb.WriteString(fmt.Sprintf("  %s\n", ex))
			}
		}
	}

	return sb.String()
}

// formatFlagLine renders a single flag as a help line.
// Example: "  --name string (REQUIRED) - Name of the provider [vsphere, ovirt]"
func formatFlagLine(f Flag, required bool) string {
	displayName := strings.ReplaceAll(f.Name, "-", "_")
	line := fmt.Sprintf("  --%s %s", displayName, f.Type)
	if required {
		line += " (REQUIRED)"
	}
	line += " - " + f.Description
	if len(f.Enum) > 0 {
		line += " [" + strings.Join(f.Enum, ", ") + "]"
	}
	return line + "\n"
}

// importantGlobalFlags lists the global flags that are relevant for MCP tool descriptions.
var importantGlobalFlags = map[string]bool{
	"namespace": true, "all-namespaces": true, "inventory-url": true, "verbose": true,
}

func (r *Registry) formatGlobalFlags(extraNames ...string) string {
	extras := make(map[string]bool, len(extraNames))
	for _, name := range extraNames {
		extras[name] = true
	}

	var sb strings.Builder
	sb.WriteString("\nCommon flags:\n")

	found := false
	for _, f := range r.GlobalFlags {
		if !importantGlobalFlags[f.Name] && !extras[f.Name] {
			continue
		}
		found = true
		displayName := strings.ReplaceAll(f.Name, "-", "_")
		sb.WriteString(fmt.Sprintf("- %s: %s\n", displayName, f.Description))
	}

	if !found {
		return ""
	}
	return sb.String()
}

// deepSiblingGroup represents a group of commands at depth >= 3 that share a common parent path.
type deepSiblingGroup struct {
	parentPath  string
	children    []string
	description string
}

// detectDeepSiblingGroups finds groups of commands at depth >= 3 that share a common
// parent path with at least 3 siblings (e.g., get/inventory/*). This is used to
// compact many inventory-style subcommands into a single summary line.
func detectDeepSiblingGroups(commands map[string]*Command, parents map[string]*Command) ([]deepSiblingGroup, map[string]bool) {
	const minGroupSize = 3

	parentChildren := make(map[string][]string)
	parentChildKeys := make(map[string][]string)

	for key := range commands {
		parts := strings.Split(key, "/")
		if len(parts) < 3 {
			continue
		}
		parentPath := strings.Join(parts[:len(parts)-1], "/")
		childName := parts[len(parts)-1]
		parentChildren[parentPath] = append(parentChildren[parentPath], childName)
		parentChildKeys[parentPath] = append(parentChildKeys[parentPath], key)
	}

	var groups []deepSiblingGroup
	groupedKeys := make(map[string]bool)

	var parentPaths []string
	for p := range parentChildren {
		parentPaths = append(parentPaths, p)
	}
	sort.Strings(parentPaths)

	for _, parentPath := range parentPaths {
		children := parentChildren[parentPath]
		if len(children) < minGroupSize {
			continue
		}

		sort.Strings(children)

		// Use parent's description if available
		desc := ""
		if parents != nil {
			if parent, ok := parents[parentPath]; ok && parent.Description != "" {
				desc = parent.Description
			}
		}
		if desc == "" {
			desc = "Get " + strings.ReplaceAll(parentPath, "/", " ") + " resources"
		}

		groups = append(groups, deepSiblingGroup{
			parentPath:  parentPath,
			children:    children,
			description: desc,
		})

		for _, key := range parentChildKeys[parentPath] {
			groupedKeys[key] = true
		}
	}

	return groups, groupedKeys
}

// detectBareParents finds commands that are structural grouping nodes rather than
// real commands. A bare parent is a command whose path is a proper prefix of another
// command's path AND that has no command-specific flags.
func detectBareParents(commands map[string]*Command) map[string]bool {
	bareParents := make(map[string]bool)

	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}

	for _, key := range keys {
		cmd := commands[key]
		// Must have no command-specific flags
		if len(cmd.Flags) > 0 {
			continue
		}

		// Check if this path is a proper prefix of any other command's path
		prefix := key + "/"
		for _, otherKey := range keys {
			if strings.HasPrefix(otherKey, prefix) {
				bareParents[key] = true
				break
			}
		}
	}

	return bareParents
}
