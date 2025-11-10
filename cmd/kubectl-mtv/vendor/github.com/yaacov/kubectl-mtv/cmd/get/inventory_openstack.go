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

// NewInventoryInstanceCmd creates the get inventory instance command
func NewInventoryInstanceCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "instance PROVIDER",
		Short:             "Get instances from a provider (openstack)",
		Long:              `Get instances from a provider (openstack)`,
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

			logNamespaceOperation("Getting instances from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListInstances(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventoryImageCmd creates the get inventory image command
func NewInventoryImageCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "image PROVIDER",
		Short:             "Get images from a provider (openstack)",
		Long:              `Get images from a provider (openstack)`,
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

			logNamespaceOperation("Getting images from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListImages(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventoryFlavorCmd creates the get inventory flavor command
func NewInventoryFlavorCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "flavor PROVIDER",
		Short:             "Get flavors from a provider (openstack)",
		Long:              `Get flavors from a provider (openstack)`,
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

			logNamespaceOperation("Getting flavors from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListFlavors(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventoryProjectCmd creates the get inventory project command
func NewInventoryProjectCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "project PROVIDER",
		Short:             "Get projects from a provider (openstack)",
		Long:              `Get projects from a provider (openstack)`,
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

			logNamespaceOperation("Getting projects from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListProjects(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventoryVolumeCmd creates the get inventory volume command
func NewInventoryVolumeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "volume PROVIDER",
		Short:             "Get volumes from a provider (openstack)",
		Long:              `Get volumes from a provider (openstack)`,
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

			logNamespaceOperation("Getting volumes from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListVolumes(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventoryVolumeTypeCmd creates the get inventory volumetype command
func NewInventoryVolumeTypeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "volumetype PROVIDER",
		Short:             "Get volume types from a provider (openstack)",
		Long:              `Get volume types from a provider (openstack)`,
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

			logNamespaceOperation("Getting volume types from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListVolumeTypes(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventorySnapshotCmd creates the get inventory snapshot command
func NewInventorySnapshotCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "snapshot PROVIDER",
		Short:             "Get snapshots from a provider (openstack)",
		Long:              `Get snapshots from a provider (openstack)`,
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

			logNamespaceOperation("Getting snapshots from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListSnapshots(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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

// NewInventorySubnetCmd creates the get inventory subnet command
func NewInventorySubnetCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	var inventoryURL string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               "subnet PROVIDER",
		Short:             "Get subnets from a provider (openstack)",
		Long:              `Get subnets from a provider (openstack)`,
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

			logNamespaceOperation("Getting subnets from provider", namespace, config.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(ctx, config.GetKubeConfigFlags(), namespace)
			}

			return inventory.ListSubnets(ctx, config.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch)
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
