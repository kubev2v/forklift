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

// NewInventoryDiskProfileCmd creates the get inventory disk-profile command
func NewInventoryDiskProfileCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:          "disk-profile",
		Short:        "Get disk profiles from a provider",
		Long:         `Get disk profiles from an oVirt provider's inventory.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
				defer cancel()
			}

			cfg := globalConfig.GetKubeConfigFlags()
			allNamespaces := globalConfig.GetAllNamespaces()
			namespace := client.ResolveNamespaceWithAllFlag(cfg, allNamespaces)

			logNamespaceOperation("Getting disk profiles from provider", namespace, allNamespaces)
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListDiskProfilesWithInsecure(ctx, cfg, provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider name")
	_ = cmd.MarkFlagRequired("provider")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'prod-.*'\")")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for provider and output format flags
	if err := cmd.RegisterFlagCompletionFunc("provider", completion.ProviderNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}

// NewInventoryNICProfileCmd creates the get inventory nic-profile command
func NewInventoryNICProfileCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:          "nic-profile",
		Short:        "Get NIC profiles from a provider",
		Long:         `Get vNIC profiles from an oVirt provider's inventory.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
				defer cancel()
			}

			cfg := globalConfig.GetKubeConfigFlags()
			allNamespaces := globalConfig.GetAllNamespaces()
			namespace := client.ResolveNamespaceWithAllFlag(cfg, allNamespaces)

			logNamespaceOperation("Getting NIC profiles from provider", namespace, allNamespaces)
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListNICProfilesWithInsecure(ctx, cfg, provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider name")
	_ = cmd.MarkFlagRequired("provider")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'prod-.*'\")")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for provider and output format flags
	if err := cmd.RegisterFlagCompletionFunc("provider", completion.ProviderNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
