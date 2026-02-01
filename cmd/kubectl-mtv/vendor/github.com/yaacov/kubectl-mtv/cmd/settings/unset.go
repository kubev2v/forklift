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

// NewUnsetCmd creates the 'settings unset' subcommand.
func NewUnsetCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset SETTING",
		Short: "Remove a ForkliftController setting (revert to default)",
		Long: `Remove a ForkliftController setting, reverting it to the default value.

This removes the setting from the ForkliftController spec, causing the controller
to use its default value instead.

Examples:
  # Remove the VDDK image setting (revert to default)
  kubectl mtv settings unset vddk_image

  # Remove extra virt-v2v arguments
  kubectl mtv settings unset virt_v2v_extra_args

  # Revert max concurrent VMs to default (20)
  kubectl mtv settings unset controller_max_vm_inflight`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: unsetArgsCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			settingName := args[0]

			opts := settings.UnsetSettingOptions{
				ConfigFlags: kubeConfigFlags,
				Name:        settingName,
				Verbosity:   globalConfig.GetVerbosity(),
			}

			if err := settings.UnsetSetting(ctx, opts); err != nil {
				return err
			}

			def := settings.GetSettingDefinition(settingName)
			if def != nil {
				fmt.Printf("Setting '%s' removed (will use default: %s)\n", settingName, settings.FormatDefault(*def))
			} else {
				fmt.Printf("Setting '%s' removed\n", settingName)
			}
			return nil
		},
	}

	return cmd
}

// unsetArgsCompletion provides completion for the unset command arguments.
func unsetArgsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for name := range settings.SupportedSettings {
		if strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}
	sort.Strings(completions)
	return completions, cobra.ShellCompDirectiveNoFileComp
}
