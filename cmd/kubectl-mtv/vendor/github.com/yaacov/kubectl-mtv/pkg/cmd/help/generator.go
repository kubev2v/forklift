package help

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SchemaVersion is the current version of the help schema format.
// Version 1.2 removes positional args, provider hints, migration hints, and LLM annotations.
const SchemaVersion = "1.2"

// MCPHiddenAnnotation is the pflag annotation key used to hide flags from
// machine-readable help (help --machine) while keeping them visible in
// human CLI --help. Flags like --watch (interactive TUI) or --vms-table
// (human table view) should be annotated with this.
const MCPHiddenAnnotation = "mcp-hidden"

// MarkMCPHidden annotates the named flags so they are excluded from machine-
// readable help output (used by the MCP server) but remain visible in normal
// CLI --help. This is for flags that are meaningful to humans but harmful or
// useless for LLM tool use (e.g. interactive TUI, human-only display modes).
func MarkMCPHidden(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			continue
		}
		if f.Annotations == nil {
			f.Annotations = make(map[string][]string)
		}
		f.Annotations[MCPHiddenAnnotation] = []string{"true"}
	}
}

// isMCPHidden returns true if the flag carries the mcp-hidden annotation.
func isMCPHidden(f *pflag.Flag) bool {
	if f.Annotations == nil {
		return false
	}
	vals, ok := f.Annotations[MCPHiddenAnnotation]
	return ok && len(vals) > 0 && vals[0] == "true"
}

// EnumValuer is an interface for flags that provide valid values.
// Custom flag types can implement this to expose their allowed values.
type EnumValuer interface {
	GetValidValues() []string
}

// Generate creates a HelpSchema from a Cobra command tree.
func Generate(rootCmd *cobra.Command, cliVersion string, opts Options) *HelpSchema {
	schema := &HelpSchema{
		Version:         SchemaVersion,
		CLIVersion:      cliVersion,
		Name:            rootCmd.Name(),
		Description:     rootCmd.Short,
		LongDescription: rootCmd.Long,
		Commands:        []Command{},
		GlobalFlags:     []Flag{},
	}

	// Walk command tree - automatically discovers all commands
	walkCommands(rootCmd, []string{}, func(cmd *cobra.Command, path []string) {
		// Skip hidden commands unless requested
		if cmd.Hidden && !opts.IncludeHidden {
			return
		}

		runnable := cmd.Runnable()

		// Include non-runnable commands only if they are at depth ≥ 2
		// (e.g., "get inventory") and have 3+ runnable children.
		// This provides description metadata for sibling-group compaction
		// without including top-level structural parents like "get" or "create".
		if !runnable {
			if len(path) < 2 {
				return
			}
			runnableChildren := 0
			for _, child := range cmd.Commands() {
				if child.Runnable() {
					runnableChildren++
				}
			}
			if runnableChildren < 3 {
				return
			}
		}

		// Apply category filter
		category := getCategory(path)
		if opts.ReadOnly && category != "read" {
			return
		}
		if opts.Write && category != "write" {
			return
		}

		c := commandToSchema(cmd, path, opts)
		c.Runnable = runnable
		schema.Commands = append(schema.Commands, c)
	})

	// Extract global flags from persistent flags
	if opts.IncludeGlobalFlags {
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden && !opts.IncludeHidden {
				return
			}
			if isMCPHidden(f) && !opts.IncludeHidden {
				return
			}
			schema.GlobalFlags = append(schema.GlobalFlags, flagToSchema(f))
		})
	}

	return schema
}

// FilterByPath filters a schema to only include commands whose path starts with
// the given prefix. For example, FilterByPath(schema, ["get"]) keeps all "get *"
// commands, and FilterByPath(schema, ["get", "plan"]) keeps only "get plan".
// Returns the number of commands remaining after filtering.
func FilterByPath(schema *HelpSchema, prefix []string) int {
	if len(prefix) == 0 {
		return len(schema.Commands)
	}

	filtered := make([]Command, 0, len(schema.Commands))
	for _, cmd := range schema.Commands {
		if len(cmd.Path) < len(prefix) {
			continue
		}
		match := true
		for i, seg := range prefix {
			if cmd.Path[i] != seg {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, cmd)
		}
	}
	schema.Commands = filtered
	return len(filtered)
}

// walkCommands recursively visits all commands in the tree.
func walkCommands(cmd *cobra.Command, path []string, visitor func(*cobra.Command, []string)) {
	visitor(cmd, path)
	for _, child := range cmd.Commands() {
		walkCommands(child, append(append([]string{}, path...), child.Name()), visitor)
	}
}

// commandToSchema converts a Cobra command to our schema format.
func commandToSchema(cmd *cobra.Command, path []string, opts Options) Command {
	c := Command{
		Name:        cmd.Name(),
		Path:        path,
		PathString:  strings.Join(path, " "),
		Description: cmd.Short,
		Usage:       cmd.UseLine(),
		Category:    getCategory(path),
		Flags:       []Flag{},
	}

	// Include verbose fields only when not in short mode
	if !opts.Short {
		c.LongDescription = cmd.Long
		c.Examples = parseExamples(cmd.Example)
	}

	// Copy aliases
	if len(cmd.Aliases) > 0 {
		c.Aliases = append([]string{}, cmd.Aliases...)
	}

	// Extract local flags (not inherited)
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden && !opts.IncludeHidden {
			return
		}
		// Skip mcp-hidden flags in machine-readable output (same condition)
		if isMCPHidden(f) && !opts.IncludeHidden {
			return
		}
		schema := flagToSchema(f)

		// Check if flag is required (MarkFlagRequired annotates the flag, not the command)
		if ann := f.Annotations; ann != nil {
			if _, ok := ann[cobra.BashCompOneRequiredFlag]; ok {
				schema.Required = true
			}
		}

		// Try to get enum values from the flag value
		if enumValuer, ok := f.Value.(EnumValuer); ok {
			schema.Enum = enumValuer.GetValidValues()
		}

		c.Flags = append(c.Flags, schema)
	})

	return c
}

// flagToSchema converts a pflag.Flag to our schema format.
func flagToSchema(f *pflag.Flag) Flag {
	flag := Flag{
		Name:        f.Name,
		Shorthand:   f.Shorthand,
		Type:        f.Value.Type(),
		Description: f.Usage,
		Hidden:      f.Hidden,
	}

	// Set default value with proper typing based on flag type
	if f.DefValue != "" {
		flagType := f.Value.Type()
		switch {
		case flagType == "bool":
			// Convert boolean strings to actual booleans
			if f.DefValue == "true" {
				flag.Default = true
			} else if f.DefValue == "false" {
				flag.Default = false
			}
		case flagType == "int" || flagType == "int8" || flagType == "int16" || flagType == "int32" || flagType == "int64":
			// Convert integer strings to numbers
			if v, err := strconv.ParseInt(f.DefValue, 10, 64); err == nil {
				flag.Default = v
			} else {
				flag.Default = f.DefValue
			}
		case flagType == "uint" || flagType == "uint8" || flagType == "uint16" || flagType == "uint32" || flagType == "uint64":
			// Convert unsigned integer strings to numbers
			if v, err := strconv.ParseUint(f.DefValue, 10, 64); err == nil {
				flag.Default = v
			} else {
				flag.Default = f.DefValue
			}
		case flagType == "float32" || flagType == "float64":
			// Convert float strings to numbers
			if v, err := strconv.ParseFloat(f.DefValue, 64); err == nil {
				flag.Default = v
			} else {
				flag.Default = f.DefValue
			}
		case strings.HasSuffix(flagType, "Slice") || strings.HasSuffix(flagType, "Array"):
			// Convert slice/array defaults to empty array or preserve value
			if f.DefValue == "[]" {
				flag.Default = []string{}
			} else {
				flag.Default = f.DefValue
			}
		default:
			// For all other types, preserve as string
			flag.Default = f.DefValue
		}
	}

	// Try to get enum values from the flag value
	if enumValuer, ok := f.Value.(EnumValuer); ok {
		flag.Enum = enumValuer.GetValidValues()
	}

	return flag
}

// getCategory determines the command category based on its path.
func getCategory(path []string) string {
	if len(path) == 0 {
		return "admin"
	}

	// Handle settings command specially - settings set/unset is write, settings get is read
	if path[0] == "settings" {
		if len(path) >= 2 && (path[1] == "set" || path[1] == "unset") {
			return "write"
		}
		return "read"
	}

	switch path[0] {
	case "get", "describe", "health":
		return "read"
	case "create", "delete", "patch", "start", "cancel", "archive", "unarchive", "cutover":
		return "write"
	default:
		return "admin"
	}
}

// parseExamples parses Cobra-style examples into our format.
// Cobra examples are typically formatted as:
//
//	# Comment describing the example
//	command args
//
// Multi-line examples using backslash continuations are joined into a single
// command string so that downstream consumers (e.g. MCP example conversion)
// see the full command with all flags:
//
//	kubectl-mtv create provider --name vsphere-prod \
//	  --type vsphere \
//	  --url https://vcenter/sdk
//
// becomes: "kubectl-mtv create provider --name vsphere-prod --type vsphere --url https://vcenter/sdk"
func parseExamples(exampleText string) []Example {
	if exampleText == "" {
		return nil
	}

	var examples []Example
	lines := strings.Split(exampleText, "\n")

	var currentDesc string
	var pendingCmd string // accumulates backslash-continued lines
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			// Flush any pending command before starting a new description
			if pendingCmd != "" {
				examples = append(examples, Example{
					Description: currentDesc,
					Command:     pendingCmd,
				})
				pendingCmd = ""
			}
			// This is a description comment (overwrites any previous unused description)
			currentDesc = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		} else if strings.HasSuffix(line, "\\") {
			// Backslash continuation: strip the trailing '\' and accumulate
			part := strings.TrimSpace(strings.TrimSuffix(line, "\\"))
			if pendingCmd == "" {
				pendingCmd = part
			} else {
				pendingCmd += " " + part
			}
		} else {
			// Final line of a command (no trailing backslash)
			if pendingCmd != "" {
				// Join with accumulated continuation lines
				pendingCmd += " " + line
				examples = append(examples, Example{
					Description: currentDesc,
					Command:     pendingCmd,
				})
				pendingCmd = ""
			} else {
				// Single-line command
				examples = append(examples, Example{
					Description: currentDesc,
					Command:     line,
				})
			}
			currentDesc = ""
		}
	}

	// Flush any trailing continued command (edge case: example ends with '\')
	if pendingCmd != "" {
		examples = append(examples, Example{
			Description: currentDesc,
			Command:     pendingCmd,
		})
	}

	return examples
}

// RequiredFlagAnnotation is the annotation key Cobra uses to mark required flags.
// This is used when checking if a flag is required via MarkFlagRequired.
var requiredFlagRegex = regexp.MustCompile(`required`)

// IsRequiredFlag checks if a flag is marked as required on a command.
func IsRequiredFlag(cmd *cobra.Command, flagName string) bool {
	f := cmd.Flag(flagName)
	if f == nil {
		return false
	}
	if f.Annotations == nil {
		return false
	}
	for key := range f.Annotations {
		if requiredFlagRegex.MatchString(key) {
			return true
		}
	}
	return false
}
