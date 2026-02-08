package describe

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewHookCmd creates the hook description command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook NAME",
		Short: "Describe a migration hook",
		Long: `Display detailed information about a migration hook.

Shows hook configuration including container image, playbook content,
service account, deadline, and status conditions.`,
		Example: `  # Describe a hook
  kubectl-mtv describe hook my-post-hook`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.HookResourceNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Get the global configuration

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			return hook.Describe(globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC())
		},
	}

	return cmd
}
