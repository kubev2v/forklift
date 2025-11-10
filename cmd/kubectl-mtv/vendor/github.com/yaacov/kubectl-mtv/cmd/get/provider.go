package get

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewProviderCmd creates the get provider command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var inventoryURL string

	cmd := &cobra.Command{
		Use:               "provider [NAME]",
		Short:             "Get providers",
		Long:              `Get providers`,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create context with 30s timeout
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			// Get optional provider name from arguments
			var providerName string
			if len(args) > 0 {
				providerName = args[0]
			}

			// If inventoryURL is empty, try to discover it
			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			// Log the operation being performed
			if providerName != "" {
				logNamespaceOperation("Getting provider", namespace, config.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting providers", namespace, config.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return provider.List(ctx, config.GetKubeConfigFlags(), namespace, inventoryURL, outputFormatFlag.GetValue(), providerName, config.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&inventoryURL, "inventory-url", "i", os.Getenv("MTV_INVENTORY_URL"), "Base URL for the inventory service")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
