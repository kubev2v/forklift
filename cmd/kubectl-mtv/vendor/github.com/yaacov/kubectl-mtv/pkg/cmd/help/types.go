// Package help provides machine-readable help output for kubectl-mtv commands.
package help

// HelpSchema represents the complete CLI help information in a machine-readable format.
type HelpSchema struct {
	// Version is the schema version (e.g., "1.0")
	Version string `json:"version" yaml:"version"`
	// CLIVersion is the kubectl-mtv version (e.g., "v0.1.59")
	CLIVersion string `json:"cli_version" yaml:"cli_version"`
	// Name is the CLI name ("kubectl-mtv")
	Name string `json:"name" yaml:"name"`
	// Description is the CLI description
	Description string `json:"description" yaml:"description"`
	// Commands contains all leaf commands
	Commands []Command `json:"commands" yaml:"commands"`
	// GlobalFlags contains flags available to all commands
	GlobalFlags []Flag `json:"global_flags" yaml:"global_flags"`
}

// Command represents a single CLI command.
type Command struct {
	// Name is the command name (last segment of path)
	Name string `json:"name" yaml:"name"`
	// Path is the full command path as array (e.g., ["get", "plan"])
	Path []string `json:"path" yaml:"path"`
	// PathString is the full command path as space-separated string (e.g., "get plan")
	PathString string `json:"path_string" yaml:"path_string"`
	// Description is the short one-line description
	Description string `json:"description" yaml:"description"`
	// LongDescription is the extended description with details
	LongDescription string `json:"long_description,omitempty" yaml:"long_description,omitempty"`
	// Usage is the usage pattern string
	Usage string `json:"usage" yaml:"usage"`
	// Aliases are alternative command names
	Aliases []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	// Category is one of: "read", "write", "admin"
	Category string `json:"category" yaml:"category"`
	// Providers lists which providers this command applies to (empty = all)
	// Extracted from [providers: x, y] hints in the command description
	Providers []string `json:"providers,omitempty" yaml:"providers,omitempty"`
	// Flags are command-specific flags
	Flags []Flag `json:"flags" yaml:"flags"`
	// PositionalArgs are required/optional positional arguments
	PositionalArgs []PositionalArg `json:"positional_args,omitempty" yaml:"positional_args,omitempty"`
	// Examples are usage examples
	Examples []Example `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// Flag represents a command-line flag.
type Flag struct {
	// Name is the long flag name (without --)
	Name string `json:"name" yaml:"name"`
	// Shorthand is the single-char shorthand (without -)
	Shorthand string `json:"shorthand,omitempty" yaml:"shorthand,omitempty"`
	// Type is one of: "bool", "string", "int", "stringArray", "duration"
	Type string `json:"type" yaml:"type"`
	// Default is the default value
	Default interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	// Description is the flag description
	Description string `json:"description" yaml:"description"`
	// Required indicates whether the flag is required
	Required bool `json:"required" yaml:"required"`
	// Enum contains allowed values (for string flags with choices)
	Enum []string `json:"enum,omitempty" yaml:"enum,omitempty"`
	// Hidden indicates whether the flag is hidden from normal help
	Hidden bool `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	// Providers lists which providers support this flag (empty = all)
	// Extracted from [providers: x, y] hints in the flag description
	Providers []string `json:"providers,omitempty" yaml:"providers,omitempty"`
	// MigrationTypes lists which migration types this flag applies to (empty = all)
	// Extracted from [migration: x, y] hints in the flag description
	MigrationTypes []string `json:"migration_types,omitempty" yaml:"migration_types,omitempty"`
}

// PositionalArg represents a positional command argument.
type PositionalArg struct {
	// Name is the argument name (usually UPPERCASE)
	Name string `json:"name" yaml:"name"`
	// Description is the argument description
	Description string `json:"description" yaml:"description"`
	// Required indicates whether the argument is required
	Required bool `json:"required" yaml:"required"`
	// Variadic indicates whether multiple values are accepted
	Variadic bool `json:"variadic,omitempty" yaml:"variadic,omitempty"`
}

// Example represents a usage example.
type Example struct {
	// Description explains what the example does
	Description string `json:"description" yaml:"description"`
	// Command is the example command
	Command string `json:"command" yaml:"command"`
}

// Options configures the help generation.
type Options struct {
	// ReadOnly includes only read-only commands when true
	ReadOnly bool
	// Write includes only write commands when true
	Write bool
	// IncludeGlobalFlags includes global flags in output (default: true)
	IncludeGlobalFlags bool
	// IncludeHidden includes hidden flags and commands
	IncludeHidden bool
}

// DefaultOptions returns the default generation options.
func DefaultOptions() Options {
	return Options{
		IncludeGlobalFlags: true,
		IncludeHidden:      false,
	}
}
