package help

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
)

// NewHelpCmd creates the help command with machine-readable output support.
func NewHelpCmd(rootCmd *cobra.Command, clientVersion string) *cobra.Command {
	var machine bool
	var readOnly bool
	var write bool
	var includeGlobalFlags bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.

Simply type kubectl-mtv help [path to command] for full details.

Use --machine to output the complete command schema in a machine-readable
format (JSON or YAML) for integration with MCP servers and automation tools.`,
		Example: `  # Get help for a command
  kubectl-mtv help get plan

  # Output complete command schema as JSON
  kubectl-mtv help --machine

  # Output schema in YAML format
  kubectl-mtv help --machine -o yaml

  # Output only read-only commands
  kubectl-mtv help --machine --read-only

  # Output only write commands
  kubectl-mtv help --machine --write`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !machine {
				// Default help behavior - show help for root or specified command
				if len(args) == 0 {
					return rootCmd.Help()
				}
				// Find the subcommand and show its help
				targetCmd, _, err := rootCmd.Find(args)
				if err != nil {
					return fmt.Errorf("unknown command %q for %q", args, rootCmd.Name())
				}
				return targetCmd.Help()
			}

			// Machine-readable output
			if readOnly && write {
				return fmt.Errorf("flags --read-only and --write are mutually exclusive")
			}

			opts := help.Options{
				ReadOnly:           readOnly,
				Write:              write,
				IncludeGlobalFlags: includeGlobalFlags,
			}

			schema := help.Generate(rootCmd, clientVersion, opts)

			return outputSchema(cmd, schema, outputFormat)
		},
	}

	cmd.Flags().BoolVar(&machine, "machine", false, "Enable machine-readable output")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format for --machine: json, yaml")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Include only read-only commands (with --machine)")
	cmd.Flags().BoolVar(&write, "write", false, "Include only write commands (with --machine)")
	cmd.Flags().BoolVar(&includeGlobalFlags, "include-global-flags", true, "Include global flags in output (with --machine)")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		// Ignore completion registration errors - not critical
		_ = err
	}

	return cmd
}

// outputSchema writes the schema to the command's output in the specified format.
func outputSchema(cmd *cobra.Command, schema *help.HelpSchema, format string) error {
	var output []byte
	var err error

	switch format {
	case "yaml":
		output, err = yaml.Marshal(schema)
	case "json":
		output, err = json.MarshalIndent(schema, "", "  ")
	default:
		return fmt.Errorf("unsupported output format: %s (use json or yaml)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}
