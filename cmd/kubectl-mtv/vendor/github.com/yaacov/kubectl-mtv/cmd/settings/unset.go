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
	var settingName string

	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Remove a ForkliftController setting (revert to default)",
		Long: `Remove a ForkliftController setting, reverting it to the default value.

This removes the setting from the ForkliftController spec, causing the controller
to use its default value instead.

Examples:
  # Remove the VDDK image setting (revert to default)
  kubectl mtv settings unset --setting vddk_image

  # Remove extra virt-v2v arguments
  kubectl mtv settings unset --setting virt_v2v_extra_args

  # Revert max concurrent VMs to default (20)
  kubectl mtv settings unset --setting controller_max_vm_inflight`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

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

	cmd.Flags().StringVar(&settingName, "setting", "", "Setting name")
	if err := cmd.MarkFlagRequired("setting"); err != nil {
		_ = err
	}

	_ = cmd.RegisterFlagCompletionFunc("setting", unsetSettingCompletion)

	return cmd
}

// unsetSettingCompletion provides completion for the --setting flag.
func unsetSettingCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	for name := range settings.SupportedSettings {
		if strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}
	sort.Strings(completions)
	return completions, cobra.ShellCompDirectiveNoFileComp
}
