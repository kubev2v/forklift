package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/settings"
	"github.com/yaacov/kubectl-mtv/pkg/util/config"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// GlobalConfigGetter is a type alias for the shared config interface.
type GlobalConfigGetter = config.GlobalConfigGetter

// NewSettingsCmd creates the settings command with subcommands.
func NewSettingsCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var allSettings bool

	cmd := &cobra.Command{
		Use:   "settings",
		Short: "View and manage ForkliftController settings",
		Long: `View and manage ForkliftController configuration settings.

This command provides access to a curated subset of ForkliftController settings
that users commonly need to configure, including:

  - VDDK image for vSphere migrations
  - Feature flags (warm migration, copy offload, live migration)
  - Performance tuning (max concurrent VMs, precopy interval)
  - Container resource settings (virt-v2v, populator)
  - Debugging options (log level)

Use --all to see all available ForkliftController settings, including advanced
options for controller, inventory, API, validation, and other components.

Examples:
  # View common settings
  kubectl mtv settings

  # View ALL settings (including advanced)
  kubectl mtv settings --all

  # View settings in YAML format
  kubectl mtv settings --output yaml

  # Get a specific setting
  kubectl mtv settings get --setting vddk_image

  # Set a value
  kubectl mtv settings set --setting vddk_image --value quay.io/myorg/vddk:8.0
  kubectl mtv settings set --setting controller_max_vm_inflight --value 30
  kubectl mtv settings set --setting feature_ocp_live_migration --value true`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default action: show all settings
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			opts := settings.GetSettingsOptions{
				ConfigFlags: kubeConfigFlags,
				AllSettings: allSettings,
			}

			settingValues, err := settings.GetSettings(ctx, opts)
			if err != nil {
				return err
			}

			return formatOutput(settingValues, outputFormatFlag.GetValue())
		},
	}

	// Add output format flag
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (json, yaml, table)")
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		// Silently ignore completion registration errors
		_ = err
	}

	// Add --all flag
	cmd.Flags().BoolVar(&allSettings, "all", false, "Show all ForkliftController settings (not just common ones)")

	// Add subcommands
	cmd.AddCommand(newGetCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(NewSetCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(NewUnsetCmd(kubeConfigFlags, globalConfig))

	return cmd
}

// newGetCmd creates the 'settings get' subcommand.
func newGetCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var allSettings bool
	var settingName string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get ForkliftController setting value(s)",
		Long: `Get the current value of one or more ForkliftController settings.

If no setting name is provided, all settings are displayed.
If a setting name is provided, only that setting's value is shown.

Use --all to see all available ForkliftController settings, including advanced
options for controller, inventory, API, validation, and other components.

Examples:
  # Get common settings
  kubectl mtv settings get

  # Get ALL settings (including advanced)
  kubectl mtv settings get --all

  # Get a specific setting
  kubectl mtv settings get --setting vddk_image
  kubectl mtv settings get --setting controller_max_vm_inflight
  kubectl mtv settings get --setting controller_container_limits_cpu`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			opts := settings.GetSettingsOptions{
				ConfigFlags: kubeConfigFlags,
				AllSettings: allSettings,
				SettingName: settingName,
			}

			settingValues, err := settings.GetSettings(ctx, opts)
			if err != nil {
				return err
			}

			// If getting a single setting, just print the value
			if opts.SettingName != "" && outputFormatFlag.GetValue() == "table" {
				if len(settingValues) > 0 {
					fmt.Println(settings.FormatValue(settingValues[0]))
				}
				return nil
			}

			return formatOutput(settingValues, outputFormatFlag.GetValue())
		},
	}

	// Add output format flag
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (json, yaml, table)")
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		_ = err
	}

	// Add --setting flag for getting a specific setting
	cmd.Flags().StringVar(&settingName, "setting", "", "Setting name to get (if not provided, shows all settings)")

	// Add --all flag
	cmd.Flags().BoolVar(&allSettings, "all", false, "Show all ForkliftController settings (not just common ones)")

	_ = cmd.RegisterFlagCompletionFunc("setting", getSettingCompletion)

	return cmd
}

// getSettingCompletion provides completion for the --setting flag in 'settings get'.
func getSettingCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	for name := range settings.SupportedSettings {
		if strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}
	sort.Strings(completions)
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// formatOutput formats the settings output.
func formatOutput(settingValues []settings.SettingValue, format string) error {
	switch format {
	case "json":
		return formatJSON(settingValues)
	case "yaml":
		return formatYAML(settingValues)
	default:
		return formatTable(settingValues)
	}
}

// formatTable formats settings as a table.
func formatTable(settingValues []settings.SettingValue) error {
	tableHeaders := []output.Header{
		{DisplayName: "CATEGORY", JSONPath: "category"},
		{DisplayName: "SETTING", JSONPath: "setting"},
		{DisplayName: "VALUE", JSONPath: "value"},
		{DisplayName: "DEFAULT", JSONPath: "default"},
	}
	items := make([]map[string]interface{}, 0, len(settingValues))
	for _, sv := range settingValues {
		items = append(items, map[string]interface{}{
			"category": string(sv.Definition.Category),
			"setting":  sv.Name,
			"value":    settings.FormatValue(sv),
			"default":  settings.FormatDefault(sv.Definition),
		})
	}

	return output.NewTablePrinter().
		WithHeaders(tableHeaders...).
		AddItems(items).
		Print()
}

// settingOutput is used for JSON/YAML output.
type settingOutput struct {
	Name        string      `json:"name" yaml:"name"`
	Value       interface{} `json:"value" yaml:"value"`
	Default     interface{} `json:"default" yaml:"default"`
	IsSet       bool        `json:"isSet" yaml:"isSet"`
	Category    string      `json:"category" yaml:"category"`
	Description string      `json:"description" yaml:"description"`
}

// formatJSON formats settings as JSON.
func formatJSON(settingValues []settings.SettingValue) error {
	output := make([]settingOutput, 0, len(settingValues))
	for _, sv := range settingValues {
		value := sv.Value
		if !sv.IsSet {
			value = nil
		}
		output = append(output, settingOutput{
			Name:        sv.Name,
			Value:       value,
			Default:     sv.Default,
			IsSet:       sv.IsSet,
			Category:    string(sv.Definition.Category),
			Description: sv.Definition.Description,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// formatYAML formats settings as YAML.
func formatYAML(settingValues []settings.SettingValue) error {
	output := make([]settingOutput, 0, len(settingValues))
	for _, sv := range settingValues {
		value := sv.Value
		if !sv.IsSet {
			value = nil
		}
		output = append(output, settingOutput{
			Name:        sv.Name,
			Value:       value,
			Default:     sv.Default,
			IsSet:       sv.IsSet,
			Category:    string(sv.Definition.Category),
			Description: sv.Definition.Description,
		})
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	fmt.Print(string(data))
	return nil
}
