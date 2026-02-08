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

// NewInventoryDatastoreCmd creates the get inventory datastore command
func NewInventoryDatastoreCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "datastore PROVIDER",
		Short:             "Get datastores from a provider " + flags.ProvidersVSphere,
		Long:              `Get datastores from a vSphere provider's inventory.`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
				defer cancel()
			}

			provider := args[0]

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			logNamespaceOperation("Getting datastores from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListDatastoresWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}

// NewInventoryResourcePoolCmd creates the get inventory resource-pool command
func NewInventoryResourcePoolCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "resource-pool PROVIDER",
		Short:             "Get resource pools from a provider " + flags.ProvidersVSphere,
		Long:              `Get resource pools from a vSphere provider's inventory.`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
				defer cancel()
			}

			provider := args[0]

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			logNamespaceOperation("Getting resource pools from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListResourcePoolsWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}

// NewInventoryFolderCmd creates the get inventory folder command
func NewInventoryFolderCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "folder PROVIDER",
		Short:             "Get folders from a provider " + flags.ProvidersVSphere,
		Long:              `Get folders from a vSphere provider's inventory.`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
				defer cancel()
			}

			provider := args[0]

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			logNamespaceOperation("Getting folders from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListFoldersWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
