package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/host"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewHostCmd creates the host description command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "host",
		Short: "Describe a migration host",
		Long: `Display detailed information about a migration host.

Shows host configuration, IP address, provider reference, and status conditions.`,
		Example: `  # Describe a host
  kubectl-mtv describe host --name esxi-host-1`,
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
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()
			return host.Describe(cmd.Context(), globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC(), inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Host name")
	_ = cmd.MarkFlagRequired("name")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HostResourceNameCompletion(kubeConfigFlags))

	return cmd
}
