package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHookCmd creates the get hook command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool

	cmd := &cobra.Command{
		Use:   "hook [NAME]",
		Short: "Get hooks",
		Long: `Get MTV hook resources from the cluster.

Hooks are custom scripts or Ansible playbooks that run at specific points during
VM migration (pre-migration or post-migration). They can be used to customize
the migration process, such as installing drivers or configuring the target VM.`,
		Example: `  # List all hooks
  kubectl-mtv get hook

  # Get a specific hook in JSON format
  kubectl-mtv get hook my-post-hook -o json

  # Watch hook status changes
  kubectl-mtv get hook -w`,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.HookResourceNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			// Get namespace from global configuration
			kubeConfigFlags := globalConfig.GetKubeConfigFlags()
			allNamespaces := globalConfig.GetAllNamespaces()
			namespace := client.ResolveNamespaceWithAllFlag(kubeConfigFlags, allNamespaces)

			// Get optional hook name from arguments
			var hookName string
			if len(args) > 0 {
				hookName = args[0]
			}

			// Log the operation being performed
			if hookName != "" {
				logNamespaceOperation("Getting hook", namespace, allNamespaces)
			} else {
				logNamespaceOperation("Getting hooks", namespace, allNamespaces)
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return hook.List(ctx, kubeConfigFlags, namespace, watch, outputFormatFlag.GetValue(), hookName, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
