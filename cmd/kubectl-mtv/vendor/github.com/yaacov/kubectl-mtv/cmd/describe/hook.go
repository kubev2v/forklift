package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewHookCmd creates the hook description command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Describe a migration hook",
		Long: `Display detailed information about a migration hook.

Shows hook configuration including container image, playbook content,
service account, deadline, and status conditions.`,
		Example: `  # Describe a hook
  kubectl-mtv describe hook --name my-post-hook`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required --name flag
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Get the global configuration

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			return hook.Describe(globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Hook name")
	_ = cmd.MarkFlagRequired("name")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HookResourceNameCompletion(kubeConfigFlags))

	return cmd
}
