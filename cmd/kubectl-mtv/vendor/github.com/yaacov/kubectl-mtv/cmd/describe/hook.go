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
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "hook NAME",
		Short:             "Describe a migration hook",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.HookResourceNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Get the global configuration
			config := getGlobalConfig()

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(config.GetKubeConfigFlags())
			return hook.Describe(config.GetKubeConfigFlags(), name, namespace, config.GetUseUTC())
		},
	}

	return cmd
}
