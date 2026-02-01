package describe

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/host"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewHostCmd creates the host description command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host NAME",
		Short: "Describe a migration host",
		Long: `Display detailed information about a migration host.

Shows host configuration, IP address, provider reference, and status conditions.`,
		Example: `  # Describe a host
  kubectl-mtv describe host esxi-host-1`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.HostResourceNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Get the global configuration

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()
			return host.Describe(cmd.Context(), globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC(), inventoryInsecureSkipTLS)
		},
	}

	return cmd
}
