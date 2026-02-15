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

// NewInventoryNamespaceCmd creates the get inventory namespace command
func NewInventoryNamespaceCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:          "namespace",
		Short:        "Get namespaces from a provider",
		Long:         `Get namespaces from an OpenShift provider's inventory.`,
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

			logNamespaceOperation("Getting namespaces from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListNamespacesWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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

// NewInventoryPVCCmd creates the get inventory pvc command
func NewInventoryPVCCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:          "pvc",
		Short:        "Get PVCs from a provider",
		Long:         `Get PersistentVolumeClaims from an OpenShift provider's inventory.`,
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

			logNamespaceOperation("Getting PVCs from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListPersistentVolumeClaimsWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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

// NewInventoryDataVolumeCmd creates the get inventory data-volume command
func NewInventoryDataVolumeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:          "data-volume",
		Short:        "Get data volumes from a provider",
		Long:         `Get DataVolumes from an OpenShift provider's inventory.`,
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

			logNamespaceOperation("Getting data volumes from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListDataVolumesWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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
