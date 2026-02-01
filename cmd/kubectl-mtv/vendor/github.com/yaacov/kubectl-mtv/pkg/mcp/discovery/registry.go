package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Registry holds discovered kubectl-mtv commands organized by read/write access.
type Registry struct {
	// ReadOnly contains commands that don't modify cluster state
	ReadOnly map[string]*Command

	// ReadWrite contains commands that modify cluster state
	ReadWrite map[string]*Command

	// GlobalFlags are flags available to all commands
	GlobalFlags []Flag

	// RootDescription is the main kubectl-mtv description
	RootDescription string
}

// NewRegistry creates a new registry by calling kubectl-mtv help --machine.
// This single call returns the complete command schema as JSON.
func NewRegistry(ctx context.Context) (*Registry, error) {
	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "kubectl-mtv", "help", "--machine")
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
		GlobalFlags:     schema.GlobalFlags,
		RootDescription: schema.Description,
	}

	// Categorize commands by read/write based on category field
	for i := range schema.Commands {
		cmd := &schema.Commands[i]
		pathKey := cmd.PathKey()

		switch cmd.Category {
		case "read":
			registry.ReadOnly[pathKey] = cmd
		default:
			// "write" and "admin" categories go to ReadWrite
			registry.ReadWrite[pathKey] = cmd
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
	var sb strings.Builder
	sb.WriteString("Execute read-only kubectl-mtv commands to query MTV resources.\n\n")
	sb.WriteString("USE THIS TOOL FOR: health checks, getting resources, describing resources, viewing settings.\n")
	sb.WriteString("Commands: health, get, describe, settings get\n\n")
	sb.WriteString("Available commands:\n")

	commands := r.ListReadOnlyCommands()
	for _, key := range commands {
		cmd := r.ReadOnly[key]
		// Format: "get plan [NAME]" -> "get plan [NAME] - Get migration plans"
		usage := formatUsageShort(cmd)
		sb.WriteString(fmt.Sprintf("- %s - %s\n", usage, cmd.Description))
	}

	sb.WriteString("\nCommon flags:\n")
	sb.WriteString("- namespace: Target Kubernetes namespace\n")
	sb.WriteString("- all_namespaces: Query across all namespaces\n")
	sb.WriteString("- output: Output format (table, json, yaml)\n")

	sb.WriteString("\nIMPORTANT: When responding, always start by showing the user the executed command from the 'command' field in the response (e.g., \"Executed: kubectl-mtv get plan -A\").\n")

	return sb.String()
}

// GenerateReadWriteDescription generates a description for the read-write tool.
func (r *Registry) GenerateReadWriteDescription() string {
	var sb strings.Builder
	sb.WriteString("Execute kubectl-mtv commands that modify cluster state.\n\n")
	sb.WriteString("WARNING: These commands create, modify, or delete resources.\n\n")
	sb.WriteString("NOTE: For read-only operations (health, get, describe, settings get), use the mtv_read tool instead.\n\n")
	sb.WriteString("Available commands:\n")

	commands := r.ListReadWriteCommands()
	for _, key := range commands {
		cmd := r.ReadWrite[key]
		usage := formatUsageShort(cmd)
		sb.WriteString(fmt.Sprintf("- %s - %s\n", usage, cmd.Description))
	}

	sb.WriteString("\nCommon flags:\n")
	sb.WriteString("- namespace: Target Kubernetes namespace\n")

	sb.WriteString("\nSecurity - Environment Variable References:\n")
	sb.WriteString("For sensitive values (passwords, tokens), you can pass environment variable references instead of actual values:\n")
	sb.WriteString("- Use ${ENV_VAR_NAME} syntax with curly braces (e.g., password: \"${VCENTER_PASSWORD}\")\n")
	sb.WriteString("- IMPORTANT: Only ${VAR} format is recognized as env var reference. Bare $VAR is treated as literal value.\n")
	sb.WriteString("- This allows passwords starting with $ (e.g., \"$ecureP@ss\") to work correctly\n")
	sb.WriteString("- The MCP server resolves the env var at execution time\n")
	sb.WriteString("- The actual secret never appears in requests or logs\n")
	sb.WriteString("- Supported flags: --password, -p, --token, -T, --offload-vsphere-password, --offload-storage-password, --target-secret-access-key\n")

	sb.WriteString("\nIMPORTANT: When responding, always start by showing the user the executed command from the 'command' field in the response (e.g., \"Executed: kubectl-mtv create plan ...\").\n")

	return sb.String()
}

// formatUsageShort returns a short usage string for a command.
// Example: "get plan [NAME]" or "create provider NAME"
func formatUsageShort(cmd *Command) string {
	path := cmd.CommandPath()
	positionalArgs := cmd.PositionalArgsString()
	if positionalArgs != "" {
		return path + " " + positionalArgs
	}
	return path
}

// BuildCommandArgs builds command-line arguments from command path, args, and flags.
func BuildCommandArgs(cmdPath string, positionalArgs []string, flags map[string]string, namespace string, allNamespaces bool) []string {
	var args []string

	// Add command path
	parts := strings.Split(cmdPath, "/")
	args = append(args, parts...)

	// Add positional arguments
	args = append(args, positionalArgs...)

	// Add namespace flags
	if allNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
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
