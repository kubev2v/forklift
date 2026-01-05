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

// NewInventoryHostCmd creates the get inventory host command
func NewInventoryHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "host PROVIDER",
		Short:             "Get hosts from a provider (ovirt, vsphere)",
		Long:              `Get hosts from a provider (ovirt, vsphere)`,
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
			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			logNamespaceOperation("Getting hosts from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListHosts(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
		},
	}

	cmd.Flags().StringVar(&inventoryURL, "inventory-url", "", "Inventory service URL")
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

// NewInventoryDataCenterCmd creates the get inventory datacenter command
func NewInventoryDataCenterCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "datacenter PROVIDER",
		Short:             "Get datacenters from a provider (ovirt, vsphere)",
		Long:              `Get datacenters from a provider (ovirt, vsphere)`,
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
			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			logNamespaceOperation("Getting datacenters from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListDataCenters(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
		},
	}

	cmd.Flags().StringVar(&inventoryURL, "inventory-url", "", "Inventory service URL")
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

// NewInventoryClusterCmd creates the get inventory cluster command
func NewInventoryClusterCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "cluster PROVIDER",
		Short:             "Get clusters from a provider (ovirt, vsphere)",
		Long:              `Get clusters from a provider (ovirt, vsphere)`,
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
			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			logNamespaceOperation("Getting clusters from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListClusters(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
		},
	}

	cmd.Flags().StringVar(&inventoryURL, "inventory-url", "", "Inventory service URL")
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

// NewInventoryDiskCmd creates the get inventory disk command
func NewInventoryDiskCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "disk PROVIDER",
		Short:             "Get disks from a provider (ovirt, vsphere)",
		Long:              `Get disks from a provider (ovirt, vsphere)`,
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
			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			logNamespaceOperation("Getting disks from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListDisks(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
		},
	}

	cmd.Flags().StringVar(&inventoryURL, "inventory-url", "", "Inventory service URL")
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
