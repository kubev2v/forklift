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
	cmd := &cobra.Command{
		Use:   "set SETTING VALUE",
		Short: "Set a ForkliftController setting value",
		Long: `Set a ForkliftController setting value.

The setting name must be one of the supported settings. Use 'kubectl mtv settings'
to see all available settings and their current values.

Value types are automatically validated:
  - Boolean settings accept: true, false, yes, no, 1, 0
  - Integer settings accept: numeric values
  - String settings accept: any value

Examples:
  # Set the VDDK image for vSphere migrations
  kubectl mtv settings set vddk_image quay.io/myorg/vddk:8.0

  # Increase maximum concurrent VM migrations
  kubectl mtv settings set controller_max_vm_inflight 30

  # Enable OpenShift cross-cluster live migration
  kubectl mtv settings set feature_ocp_live_migration true

  # Increase virt-v2v memory limit for large VMs
  kubectl mtv settings set virt_v2v_container_limits_memory 16Gi

  # Set a value starting with -- (use -- to stop flag parsing)
  kubectl mtv settings set virt_v2v_extra_args -- --machine-readable`,
		Args:              cobra.ExactArgs(2),
		SilenceUsage:      true,
		ValidArgsFunction: setArgsCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			settingName := args[0]
			settingValue := args[1]

			opts := settings.SetSettingOptions{
				ConfigFlags: kubeConfigFlags,
				Name:        settingName,
				Value:       settingValue,
				Verbosity:   globalConfig.GetVerbosity(),
			}

			if err := settings.SetSetting(ctx, opts); err != nil {
				return err
			}

			fmt.Printf("Setting '%s' updated to '%s'\n", settingName, settingValue)
			return nil
		},
	}

	return cmd
}

// setArgsCompletion provides completion for the set command arguments.
func setArgsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// First argument: setting name
		var completions []string
		for name := range settings.SupportedSettings {
			if strings.HasPrefix(name, toComplete) {
				completions = append(completions, name)
			}
		}
		sort.Strings(completions)
		return completions, cobra.ShellCompDirectiveNoFileComp
	case 1:
		// Second argument: value
		def := settings.GetSettingDefinition(args[0])
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
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
