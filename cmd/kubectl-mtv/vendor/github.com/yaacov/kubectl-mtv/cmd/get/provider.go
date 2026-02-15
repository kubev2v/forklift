package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewProviderCmd creates the get provider command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool

	var providerName string
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Get providers",
		Long: `Get MTV providers from the cluster.

Providers represent source (oVirt, vSphere, OpenStack, OVA, EC2) or target (OpenShift)
environments for VM migrations. Lists all providers or retrieves details for a specific one.`,
		Example: `  # List all providers
  kubectl-mtv get providers

  # List providers across all namespaces
  kubectl-mtv get providers --all-namespaces

  # Get provider details in YAML format
  kubectl-mtv get provider --name vsphere-prod --output yaml

  # Watch provider status changes
  kubectl-mtv get providers --watch`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			kubeConfigFlags := globalConfig.GetKubeConfigFlags()
			allNamespaces := globalConfig.GetAllNamespaces()
			namespace := client.ResolveNamespaceWithAllFlag(kubeConfigFlags, allNamespaces)

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			// Log the operation being performed
			if providerName != "" {
				logNamespaceOperation("Getting provider", namespace, allNamespaces)
			} else {
				logNamespaceOperation("Getting providers", namespace, allNamespaces)
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return provider.List(ctx, kubeConfigFlags, namespace, inventoryURL, watch, outputFormatFlag.GetValue(), providerName, inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().StringVarP(&providerName, "name", "M", "", "Provider name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for name and output format flags
	if err := cmd.RegisterFlagCompletionFunc("name", completion.ProviderNameCompletion(kubeConfigFlags)); err != nil {
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
