package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHookCmd creates the hook description command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Describe a migration hook",
		Long: `Display detailed information about a migration hook.

Shows hook configuration including container image, playbook content,
service account, deadline, and status conditions.`,
		Example: `  # Describe a hook
  kubectl-mtv describe hook --name my-post-hook`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.ResolveNameArg(&name, args); err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			return hook.Describe(globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC(), outputFormatFlag.GetValue())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Hook name")
	flags.MarkRequiredForMCP(cmd, "name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HookResourceNameCompletion(kubeConfigFlags))
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}
