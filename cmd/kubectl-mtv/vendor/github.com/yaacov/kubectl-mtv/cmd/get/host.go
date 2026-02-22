package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/host"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHostCmd creates the get host command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool

	var hostName string
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Get hosts",
		Long: `Get MTV host resources from the cluster.

Host resources represent ESXi hosts for vSphere migrations or hypervisor hosts
for oVirt migrations. They store host-specific credentials and configuration.`,
		Example: `  # List all hosts
  kubectl-mtv get hosts

  # Get a specific host in YAML format
  kubectl-mtv get host --name esxi-host-1 --output yaml

  # Watch host status changes
  kubectl-mtv get hosts --watch`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			// Get namespace from global configuration
			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			// Log the operation being performed
			if hostName != "" {
				logNamespaceOperation("Getting host", namespace, globalConfig.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting hosts", namespace, globalConfig.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return host.List(ctx, globalConfig.GetKubeConfigFlags(), namespace, watch, outputFormatFlag.GetValue(), hostName, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&hostName, "name", "M", "", "Host name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")
	help.MarkMCPHidden(cmd, "watch")

	// Add completion for name and output format flags
	if err := cmd.RegisterFlagCompletionFunc("name", completion.HostResourceNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}
	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
