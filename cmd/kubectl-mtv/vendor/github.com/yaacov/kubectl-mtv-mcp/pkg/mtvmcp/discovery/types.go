// Package discovery provides dynamic command discovery from kubectl-mtv help output.
package discovery

import "strings"

// HelpSchema matches kubectl-mtv help --machine output
type HelpSchema struct {
	Version     string    `json:"version"`
	CLIVersion  string    `json:"cli_version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Commands    []Command `json:"commands"`
	GlobalFlags []Flag    `json:"global_flags"`
}

// Command represents a kubectl-mtv command discovered from help --machine output.
type Command struct {
	// Name is the command name (e.g., "plan", "provider")
	Name string `json:"name"`

	// Path is the complete command path as array (e.g., ["get", "inventory", "vm"])
	Path []string `json:"path"`

	// PathString is the full command path as space-separated string
	PathString string `json:"path_string"`

	// Description is the command's short description
	Description string `json:"description"`

	// LongDescription is the extended description with details
	LongDescription string `json:"long_description,omitempty"`

	// Usage is the usage pattern (e.g., "kubectl-mtv get inventory vm PROVIDER [flags]")
	Usage string `json:"usage"`

	// Aliases are alternative names for the command
	Aliases []string `json:"aliases,omitempty"`

	// Category is one of: "read", "write", "admin"
	Category string `json:"category"`

	// Flags are the command-specific flags
	Flags []Flag `json:"flags"`

	// PositionalArgs are required/optional positional arguments
	PositionalArgs []Arg `json:"positional_args,omitempty"`
}

// Flag represents a command-line flag discovered from help --machine output.
type Flag struct {
	// Name is the long flag name without dashes (e.g., "output")
	Name string `json:"name"`

	// Shorthand is the single-character shorthand without dash (e.g., "o")
	Shorthand string `json:"shorthand,omitempty"`

	// Type is the flag type: "bool", "string", "int", "stringArray", "duration"
	Type string `json:"type"`

	// Default is the default value if specified
	Default any `json:"default,omitempty"`

	// Description is the flag's help text
	Description string `json:"description"`

	// Required indicates if the flag is required
	Required bool `json:"required"`

	// Enum contains allowed values for string flags
	Enum []string `json:"enum,omitempty"`

	// Hidden indicates if the flag is hidden from normal help
	Hidden bool `json:"hidden,omitempty"`
}

// Arg represents a positional argument.
type Arg struct {
	// Name is the argument name (usually UPPERCASE)
	Name string `json:"name"`

	// Description is the argument description
	Description string `json:"description"`

	// Required indicates if the argument is required
	Required bool `json:"required"`

	// Variadic indicates if multiple values are accepted
	Variadic bool `json:"variadic,omitempty"`
}

// CommandPath returns the full command path as a string (e.g., "get inventory vm")
func (c *Command) CommandPath() string {
	if c.PathString != "" {
		return c.PathString
	}
	return strings.Join(c.Path, " ")
}

// PathKey returns a key suitable for map lookups (e.g., "get/inventory/vm")
func (c *Command) PathKey() string {
	return strings.Join(c.Path, "/")
}

// PositionalArgsString returns positional args as a formatted string for display.
// Example: "[NAME]" or "PROVIDER"
func (c *Command) PositionalArgsString() string {
	var args []string
	for _, arg := range c.PositionalArgs {
		if arg.Required {
			args = append(args, arg.Name)
		} else {
			args = append(args, "["+arg.Name+"]")
		}
	}
	return strings.Join(args, " ")
}
