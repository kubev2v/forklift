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

	// ReadOnlyOrder preserves the original registration order of read-only
	// commands from help --machine. This is used for example selection so
	// the first command per group matches the developer's intended ordering.
	ReadOnlyOrder []string

	// ReadWrite contains commands that modify cluster state
	ReadWrite map[string]*Command

	// ReadWriteOrder preserves the original registration order of read-write commands.
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

// GetCommand returns a command by its path key (e.g., "get/plan").
func (r *Registry) GetCommand(pathKey string) *Command {
	if cmd, ok := r.ReadOnly[pathKey]; ok {
		return cmd
	}
	if cmd, ok := r.ReadWrite[pathKey]; ok {
		return cmd
	}
	return nil
}

// GetCommandByPath returns a command by its path slice.
func (r *Registry) GetCommandByPath(path []string) *Command {
	key := strings.Join(path, "/")
	return r.GetCommand(key)
}

// ListReadOnlyCommands returns sorted list of read-only command paths.
func (r *Registry) ListReadOnlyCommands() []string {
	var commands []string
	for key := range r.ReadOnly {
		commands = append(commands, key)
	}
	sort.Strings(commands)
	return commands
}

// ListReadWriteCommands returns sorted list of read-write command paths.
func (r *Registry) ListReadWriteCommands() []string {
	var commands []string
	for key := range r.ReadWrite {
		commands = append(commands, key)
	}
	sort.Strings(commands)
	return commands
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

// GenerateReadOnlyDescription generates a description for the read-only tool.
func (r *Registry) GenerateReadOnlyDescription() string {
	roots := uniqueRootVerbs(r.ReadOnly)

	var sb strings.Builder
	sb.WriteString("MTV (Migration Toolkit for Virtualization) migrates VMs from VMware vSphere, oVirt, OpenStack, and Amazon EC2 into OpenShift Virtualization (KubeVirt).\n\n")
	sb.WriteString("Execute read-only kubectl-mtv commands to query MTV resources.\n\n")
	sb.WriteString(fmt.Sprintf("Commands: %s\n\n", strings.Join(roots, ", ")))
	sb.WriteString("Available commands:\n")

	commands := r.ListReadOnlyCommands()
	for _, key := range commands {
		cmd := r.ReadOnly[key]
		// Format: "get plan [NAME]" -> "get plan [NAME] - Get migration plans"
		usage := formatUsageShort(cmd)
		sb.WriteString(fmt.Sprintf("- %s - %s\n", usage, cmd.Description))
	}

	sb.WriteString(r.formatGlobalFlags())

	// Include extended notes from commands that have substantial LongDescription
	sb.WriteString(r.generateReadOnlyCommandNotes())

	sb.WriteString("\nEnvironment Variable References:\n")
	sb.WriteString("- Use ${ENV_VAR_NAME} syntax to pass environment variable references as flag values\n")

	return sb.String()
}

// GenerateReadWriteDescription generates a description for the read-write tool.
func (r *Registry) GenerateReadWriteDescription() string {
	var sb strings.Builder
	sb.WriteString("Execute kubectl-mtv commands that modify cluster state.\n\n")
	sb.WriteString("WARNING: These commands create, modify, or delete resources.\n\n")
	sb.WriteString("Typical migration workflow:\n")
	sb.WriteString("1. Set namespace for the migration\n")
	sb.WriteString("2. Check existing providers with mtv_read \"get provider\"; create via mtv_write \"create provider\" only if needed\n")
	sb.WriteString("3. Browse VMs with mtv_read \"get inventory vm\" + TSL queries\n")
	sb.WriteString("4. Create a migration plan (network/storage mappings are auto-generated; use --network-pairs/--storage-pairs to override)\n")
	sb.WriteString("5. Start the plan\n")
	sb.WriteString("6. Monitor with mtv_read \"get plan\", debug with kubectl_logs\n\n")
	readRoots := uniqueRootVerbs(r.ReadOnly)
	sb.WriteString(fmt.Sprintf("NOTE: For read-only operations (%s), use the mtv_read tool instead.\n\n", strings.Join(readRoots, ", ")))
	sb.WriteString("Available commands:\n")

	commands := r.ListReadWriteCommands()
	for _, key := range commands {
		cmd := r.ReadWrite[key]
		usage := formatUsageShort(cmd)
		sb.WriteString(fmt.Sprintf("- %s - %s\n", usage, cmd.Description))
	}

	sb.WriteString(r.formatGlobalFlags())

	// Append per-command flag reference for complex write commands
	sb.WriteString(r.generateFlagReference())

	sb.WriteString("\nEnvironment Variable References:\n")
	sb.WriteString("You can pass environment variable references instead of literal values for any flag:\n")
	sb.WriteString("- Use ${ENV_VAR_NAME} syntax with curly braces (e.g., url: \"${GOVC_URL}\", password: \"${VCENTER_PASSWORD}\")\n")
	sb.WriteString("- Env vars can be embedded in strings (e.g., url: \"${GOVC_URL}/sdk\", endpoint: \"https://${HOST}:${PORT}/api\")\n")
	sb.WriteString("- IMPORTANT: Only ${VAR} format is recognized as env var reference. Bare $VAR is treated as literal value.\n")
	sb.WriteString("- The MCP server resolves the env var at execution time\n")
	sb.WriteString("- Sensitive values (passwords, tokens) are masked in command output for security\n")

	return sb.String()
}

// GenerateMinimalReadOnlyDescription generates a minimal description for the read-only tool.
// It puts the command list first (most critical for the LM), followed by examples
// and a hint to use mtv_help. Domain context and conventions are omitted to stay
// within client description limits and avoid noise for smaller models.
func (r *Registry) GenerateMinimalReadOnlyDescription() string {
	var sb strings.Builder

	sb.WriteString("MTV (Migration Toolkit for Virtualization) migrates VMs from VMware vSphere, oVirt, OpenStack, and Amazon EC2 into OpenShift Virtualization (KubeVirt).\n")
	sb.WriteString("\nQuery MTV resources (read-only).\n")
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
	examples := r.collectOrderedExamples(r.ReadOnly, r.ReadOnlyOrder, 6)
	if len(examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, ex := range examples {
			sb.WriteString(fmt.Sprintf("  %s\n", ex))
		}
	}

	sb.WriteString(r.formatGlobalFlags())

	sb.WriteString("\nDefault output is table. For structured data, use flags: {output: \"json\"} and fields: [\"name\", \"status\"] to limit response size.\n")
	sb.WriteString("IMPORTANT: 'fields' is a TOP-LEVEL parameter, NOT inside flags. Example: {command: \"get plan\", flags: {output: \"json\"}, fields: [\"name\", \"status\"]}\n")
	sb.WriteString("Use mtv_help for flags, TSL query syntax, and examples.\n")
	sb.WriteString("IMPORTANT: Before writing inventory queries, call mtv_help(\"tsl\") to learn available fields per provider and query syntax.\n")
	return sb.String()
}

// GenerateMinimalReadWriteDescription generates a minimal description for the read-write tool.
// It puts the command list first (most critical for the LM), followed by examples
// and a hint to use mtv_help. Domain context and conventions are omitted to stay
// within client description limits and avoid noise for smaller models.
func (r *Registry) GenerateMinimalReadWriteDescription() string {
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
	sb.WriteString("6. Monitor with mtv_read \"get plan\", debug with kubectl_logs\n")
	sb.WriteString("\nCommands:\n")

	commands := r.ListReadWriteCommands()
	for _, key := range commands {
		if bareParents[key] {
			continue
		}
		cmd := r.ReadWrite[key]
		usage := formatUsageShort(cmd)
		sb.WriteString(fmt.Sprintf("  %s - %s\n", usage, cmd.Description))
	}

	examples := r.collectOrderedExamples(r.ReadWrite, r.ReadWriteOrder, 8)
	if len(examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, ex := range examples {
			sb.WriteString(fmt.Sprintf("  %s\n", ex))
		}
	}

	sb.WriteString(r.formatGlobalFlags())

	sb.WriteString("\nCall mtv_help before create/patch to learn required flags, TSL (Tree Search Language) query syntax, and KARL (affinity/anti-affinity rule) syntax.\n")
	return sb.String()
}

// ultraMinimalReadCommands lists the most commonly used read commands for the
// ultra-minimal description. Only these are shown to very small models.
var ultraMinimalReadCommands = map[string]bool{
	"get/plan":                true,
	"get/provider":            true,
	"describe/plan":           true,
	"get/inventory/vm":        true,
	"get/mapping":             true,
	"health":                  true,
	"settings/get":            true,
	"get/inventory/network":   true,
	"get/inventory/datastore": true,
}

// ultraMinimalWriteCommands lists the most commonly used write commands.
var ultraMinimalWriteCommands = map[string]bool{
	"create/provider":        true,
	"create/plan":            true,
	"start/plan":             true,
	"delete/plan":            true,
	"delete/provider":        true,
	"patch/plan":             true,
	"create/mapping/network": true,
	"create/mapping/storage": true,
}

// GenerateUltraMinimalReadOnlyDescription generates the shortest possible description
// for the read-only tool, optimized for very small models (< 8B parameters).
// It lists only the most common commands, 2 examples, and omits flags/workflow/notes.
func (r *Registry) GenerateUltraMinimalReadOnlyDescription() string {
	var sb strings.Builder

	sb.WriteString("Query MTV migration resources (read-only).\n")
	sb.WriteString("\nCommands:\n")

	// List only ultra-minimal commands that exist in the registry
	commands := r.ListReadOnlyCommands()
	for _, key := range commands {
		if !ultraMinimalReadCommands[key] {
			continue
		}
		cmd := r.ReadOnly[key]
		sb.WriteString(fmt.Sprintf("  %s - %s\n", cmd.CommandPath(), cmd.Description))
	}

	// If we have inventory commands not in the explicit list, mention them as a group
	hasInventory := false
	for key := range r.ReadOnly {
		if strings.HasPrefix(key, "get/inventory/") && !ultraMinimalReadCommands[key] {
			hasInventory = true
			break
		}
	}
	if hasInventory {
		sb.WriteString("  get inventory RESOURCE - Get other inventory resources (disk, host, cluster, ...)\n")
	}

	sb.WriteString("\nExamples:\n")
	sb.WriteString("  {command: \"get plan\", flags: {namespace: \"demo\"}}\n")
	sb.WriteString("  {command: \"get inventory vm\", flags: {provider: \"vsphere-prod\", namespace: \"demo\"}}\n")
	sb.WriteString("\nUse mtv_help for flags and query syntax.\n")

	return sb.String()
}

// GenerateUltraMinimalReadWriteDescription generates the shortest possible description
// for the read-write tool, optimized for very small models (< 8B parameters).
func (r *Registry) GenerateUltraMinimalReadWriteDescription() string {
	var sb strings.Builder

	sb.WriteString("Create, modify, or delete MTV migration resources.\n")
	sb.WriteString("\nCommands:\n")

	commands := r.ListReadWriteCommands()
	for _, key := range commands {
		if !ultraMinimalWriteCommands[key] {
			continue
		}
		cmd := r.ReadWrite[key]
		sb.WriteString(fmt.Sprintf("  %s - %s\n", cmd.CommandPath(), cmd.Description))
	}

	sb.WriteString("\nExamples:\n")
	sb.WriteString("  {command: \"create plan\", flags: {name: \"my-plan\", source: \"vsphere-prod\", target: \"host\", vms: \"web-server\", namespace: \"demo\"}}\n")
	sb.WriteString("  {command: \"start plan\", flags: {name: \"my-plan\", namespace: \"demo\"}}\n")
	sb.WriteString("\nCall mtv_help before create/patch to learn required flags.\n")

	return sb.String()
}

// generateFlagReference builds a concise per-command flag reference for write commands.
// It includes all flags for commands that have required flags or many flags (complex commands),
// so the LLM can construct valid calls without guessing flag names.
func (r *Registry) generateFlagReference() string {
	var sb strings.Builder

	// Collect commands that need flag documentation:
	// 1. Commands with any required flags (these fail 100% without flag knowledge)
	// 2. Key complex commands (create/patch provider, create plan, create mapping)
	type commandEntry struct {
		pathKey string
		cmd     *Command
	}

	// Get sorted list of write commands
	keys := r.ListReadWriteCommands()

	// First pass: identify commands with required flags or many flags
	var flaggedCommands []commandEntry
	for _, key := range keys {
		cmd := r.ReadWrite[key]
		if cmd == nil || len(cmd.Flags) == 0 {
			continue
		}

		hasRequired := false
		for _, f := range cmd.Flags {
			if f.Required {
				hasRequired = true
				break
			}
		}

		// Include if: has required flags, or is a complex command (>5 flags)
		if hasRequired || len(cmd.Flags) > 5 {
			flaggedCommands = append(flaggedCommands, commandEntry{key, cmd})
		}
	}

	if len(flaggedCommands) == 0 {
		return ""
	}

	sb.WriteString("\nFlag reference for complex commands:\n")

	for _, entry := range flaggedCommands {
		cmd := entry.cmd
		cmdPath := strings.ReplaceAll(entry.pathKey, "/", " ")

		// Include the command's LongDescription when available, as it may contain
		// syntax references (e.g., KARL affinity syntax, TSL query language)
		if cmd.LongDescription != "" {
			sb.WriteString(fmt.Sprintf("\n%s notes:\n%s\n", cmdPath, cmd.LongDescription))
		}

		sb.WriteString(fmt.Sprintf("\n%s flags:\n", cmdPath))

		for _, f := range cmd.Flags {
			if f.Hidden {
				continue
			}

			// Format: "  --name (type) - description [REQUIRED] [enum: a|b|c]"
			line := fmt.Sprintf("  --%s", f.Name)

			// Add type for non-bool flags
			if f.Type != "bool" {
				line += fmt.Sprintf(" (%s)", f.Type)
			}

			line += fmt.Sprintf(" - %s", f.Description)

			if f.Required {
				line += " [REQUIRED]"
			}

			if len(f.Enum) > 0 {
				line += fmt.Sprintf(" [enum: %s]", strings.Join(f.Enum, "|"))
			}

			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

// generateReadOnlyCommandNotes includes LongDescription from read-only commands
// that have substantial documentation (e.g., query language syntax references).
// This surfaces documentation that was added to Cobra Long descriptions into the
// MCP tool description, so AI clients can discover syntax without external docs.
func (r *Registry) generateReadOnlyCommandNotes() string {
	var sb strings.Builder

	// Minimum length threshold to avoid including trivial one-liner descriptions
	const minLongDescLength = 200

	commands := r.ListReadOnlyCommands()
	var hasNotes bool

	for _, key := range commands {
		cmd := r.ReadOnly[key]
		if cmd == nil || len(cmd.LongDescription) < minLongDescLength {
			continue
		}

		if !hasNotes {
			sb.WriteString("\nCommand notes:\n")
			hasNotes = true
		}

		cmdPath := strings.ReplaceAll(key, "/", " ")
		sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", cmdPath, cmd.LongDescription))
	}

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

// uniqueRootVerbs extracts the unique first path element from a set of commands
// and returns them sorted. For example, commands "get/plan", "get/provider",
// "describe/plan", "health" produce ["describe", "get", "health"].
func uniqueRootVerbs(commands map[string]*Command) []string {
	seen := make(map[string]bool)
	for key := range commands {
		parts := strings.SplitN(key, "/", 2)
		seen[parts[0]] = true
	}

	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
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

// formatUsageShort returns a short usage string for a command.
func formatUsageShort(cmd *Command) string {
	return cmd.CommandPath()
}

// BuildCommandArgs builds command-line arguments from command path and flags.
func BuildCommandArgs(cmdPath string, flags map[string]string, namespace string, allNamespaces bool) []string {
	var args []string

	// Add command path
	parts := strings.Split(cmdPath, "/")
	args = append(args, parts...)

	// Add namespace flags
	if allNamespaces {
		args = append(args, "--all-namespaces")
	} else if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	// Add other flags
	for name, value := range flags {
		if name == "namespace" || name == "all_namespaces" {
			continue // Already handled
		}
		if value == "true" {
			// Boolean flag
			args = append(args, "--"+name)
		} else if value == "false" {
			// Skip false boolean flags
			continue
		} else if value != "" {
			// String/int flag with value
			args = append(args, "--"+name, value)
		}
	}

	return args
}
