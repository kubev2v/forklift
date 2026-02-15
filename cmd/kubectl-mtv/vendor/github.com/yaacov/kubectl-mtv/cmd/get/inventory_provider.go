package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewInventoryProviderCmd creates the get inventory provider command
func NewInventoryProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var providerName string

	cmd := &cobra.Command{
		Use:          "provider",
		Short:        "Get inventory information from providers",
		Long:         `Get inventory information from providers including resource counts and provider status`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
				defer cancel()
			}

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			if providerName != "" {
				logNamespaceOperation("Getting inventory from provider", namespace, globalConfig.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting inventory from all providers", namespace, globalConfig.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListProvidersWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), providerName, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().StringVarP(&providerName, "name", "M", "", "Provider name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'prod-.*'\")")
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
