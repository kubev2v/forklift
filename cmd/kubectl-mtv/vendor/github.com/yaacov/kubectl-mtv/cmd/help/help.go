package help

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
)

// NewHelpCmd creates the help command with machine-readable output support.
func NewHelpCmd(rootCmd *cobra.Command, clientVersion string) *cobra.Command {
	var machine bool
	var short bool
	var readOnly bool
	var write bool
	var includeGlobalFlags bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.

Simply type kubectl-mtv help [path to command] for full details.

Use --machine to output the command schema in a machine-readable format
(JSON or YAML) for integration with MCP servers and automation tools.
You can scope --machine output to a specific command or subtree by passing
the command path (e.g., help --machine get plan). Use --short with --machine
to omit long descriptions and examples for a condensed view.

Help topics are also available for domain-specific languages:
  tsl   - Tree Search Language query syntax reference
  karl  - Kubernetes Affinity Rule Language syntax reference`,
		Example: `  # Get help for a command
  kubectl-mtv help get plan

  # Learn about the TSL query language
  kubectl-mtv help tsl

  # Learn about the KARL affinity syntax
  kubectl-mtv help karl

  # Output complete command schema as JSON
  kubectl-mtv help --machine

  # Output schema for a single command
  kubectl-mtv help --machine get plan

  # Output schema for all "get" commands
  kubectl-mtv help --machine get

  # Condensed schema without long descriptions or examples
  kubectl-mtv help --machine --short

  # Output schema in YAML format
  kubectl-mtv help --machine --output yaml

  # Output only read-only commands
  kubectl-mtv help --machine --read-only

  # Output only write commands
  kubectl-mtv help --machine --write

  # Get TSL reference in machine-readable format
  kubectl-mtv help --machine tsl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for help topics (e.g., "help tsl", "help karl")
			if len(args) > 0 {
				if topic := help.GetTopic(args[0]); topic != nil {
					if machine {
						return outputTopic(cmd, topic, outputFormat)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n%s\n", topic.Short, topic.Content)
					return nil
				}
			}

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
				Short:              short,
			}

			schema := help.Generate(rootCmd, clientVersion, opts)

			// Filter to a specific command subtree if args are provided
			if len(args) > 0 {
				if n := help.FilterByPath(schema, args); n == 0 {
					return fmt.Errorf("unknown command %q for %q", strings.Join(args, " "), rootCmd.Name())
				}
			}

			return outputSchema(cmd, schema, outputFormat)
		},
	}

	cmd.Flags().BoolVar(&machine, "machine", false, "Enable machine-readable output")
	cmd.Flags().BoolVar(&short, "short", false, "Omit long descriptions and examples from machine output (with --machine)")
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

// outputTopic writes a help topic to the command's output in the specified format.
func outputTopic(cmd *cobra.Command, topic *help.Topic, format string) error {
	var output []byte
	var err error

	switch format {
	case "yaml":
		output, err = yaml.Marshal(topic)
	case "json":
		output, err = json.MarshalIndent(topic, "", "  ")
	default:
		return fmt.Errorf("unsupported output format: %s (use json or yaml)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal topic: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
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
