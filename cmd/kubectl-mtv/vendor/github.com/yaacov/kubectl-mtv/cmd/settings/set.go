package settings

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/settings"
)

// NewSetCmd creates the 'settings set' subcommand.
func NewSetCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var settingNames []string
	var settingValues []string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set one or more ForkliftController setting values",
		Long: `Set one or more ForkliftController setting values.

The setting name must be one of the supported settings. Use 'kubectl mtv settings'
to see all available settings and their current values.

Multiple --setting/--value pairs can be specified to update several settings in a
single Kubernetes patch operation, avoiding multiple reconciliation cycles.

Value types are automatically validated:
  - Boolean settings accept: true, false, yes, no, 1, 0
  - Integer settings accept: numeric values
  - String settings accept: any value

Examples:
  # Set the VDDK image for vSphere migrations
  kubectl mtv settings set --setting vddk_image --value quay.io/myorg/vddk:8.0

  # Increase maximum concurrent VM migrations
  kubectl mtv settings set --setting controller_max_vm_inflight --value 30

  # Enable OpenShift cross-cluster live migration
  kubectl mtv settings set --setting feature_ocp_live_migration --value true

  # Increase virt-v2v memory limit for large VMs
  kubectl mtv settings set --setting virt_v2v_container_limits_memory --value 16Gi

  # Set multiple settings at once (single reconciliation cycle)
  kubectl mtv settings set --setting feature_mcp_server --value true \
                           --setting mcp_server_lightspeed_set_mcp_gate --value true

  # Set a value starting with -- (use -- to stop flag parsing)
  kubectl mtv settings set --setting virt_v2v_extra_args --value --machine-readable`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(settingNames) != len(settingValues) {
				return fmt.Errorf("number of --setting flags (%d) must match number of --value flags (%d)", len(settingNames), len(settingValues))
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			opts := settings.SetSettingsOptions{
				ConfigFlags: kubeConfigFlags,
				Names:       settingNames,
				Values:      settingValues,
				Verbosity:   globalConfig.GetVerbosity(),
			}

			if err := settings.SetSettings(ctx, opts); err != nil {
				return err
			}

			for i, name := range settingNames {
				fmt.Printf("Setting '%s' updated to '%s'\n", name, settingValues[i])
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&settingNames, "setting", nil, "Setting name (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&settingValues, "value", nil, "Setting value (can be specified multiple times)")

	_ = cmd.MarkFlagRequired("setting")
	_ = cmd.MarkFlagRequired("value")

	_ = cmd.RegisterFlagCompletionFunc("setting", setSettingCompletion)
	_ = cmd.RegisterFlagCompletionFunc("value", setValueCompletion)

	return cmd
}

// setSettingCompletion provides completion for the --setting flag.
func setSettingCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	for name := range settings.SupportedSettings {
		if strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}
	sort.Strings(completions)
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// setValueCompletion provides completion for the --value flag based on the --setting flag.
func setValueCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	settingNameList, _ := cmd.Flags().GetStringArray("setting")
	if len(settingNameList) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	// Use the last --setting value for context-sensitive completion
	lastSettingName := settingNameList[len(settingNameList)-1]
	def := settings.GetSettingDefinition(lastSettingName)
	if def == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Provide value completions based on type
	switch def.Type {
	case settings.TypeBool:
		return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
	case settings.TypeInt:
		// For integers, suggest common values based on the setting
		switch def.Name {
		case "controller_max_vm_inflight":
			return []string{"10", "20", "30", "50", "100"}, cobra.ShellCompDirectiveNoFileComp
		case "controller_precopy_interval":
			return []string{"30", "60", "120", "180"}, cobra.ShellCompDirectiveNoFileComp
		case "controller_log_level":
			return []string{"0", "1", "2", "3", "4", "5"}, cobra.ShellCompDirectiveNoFileComp
		}
	case settings.TypeString:
		// For string settings like resource limits, suggest common values
		switch def.Name {
		case "virt_v2v_container_limits_cpu":
			return []string{"2000m", "4000m", "6000m", "8000m"}, cobra.ShellCompDirectiveNoFileComp
		case "virt_v2v_container_limits_memory":
			return []string{"4Gi", "8Gi", "12Gi", "16Gi", "32Gi"}, cobra.ShellCompDirectiveNoFileComp
		case "virt_v2v_container_requests_cpu":
			return []string{"500m", "1000m", "2000m"}, cobra.ShellCompDirectiveNoFileComp
		case "virt_v2v_container_requests_memory":
			return []string{"512Mi", "1Gi", "2Gi", "4Gi"}, cobra.ShellCompDirectiveNoFileComp
		case "populator_container_limits_cpu":
			return []string{"500m", "1000m", "2000m"}, cobra.ShellCompDirectiveNoFileComp
		case "populator_container_limits_memory":
			return []string{"512Mi", "1Gi", "2Gi"}, cobra.ShellCompDirectiveNoFileComp
		case "populator_container_requests_cpu":
			return []string{"50m", "100m", "200m"}, cobra.ShellCompDirectiveNoFileComp
		case "populator_container_requests_memory":
			return []string{"256Mi", "512Mi", "1Gi"}, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}
