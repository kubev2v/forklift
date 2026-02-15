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
func NewInventoryHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "host",
		Short: "Get hosts from a provider",
		Long: `Get hypervisor hosts from a provider's inventory.

Lists ESXi hosts (vSphere) or hypervisor hosts (oVirt) from the source provider.
Host information is useful for planning migrations and understanding the source environment.`,
		Example: `  # Filter hosts by cluster
  kubectl-mtv get inventory hosts --provider vsphere-prod --query "where cluster = 'production'"

  # List all hosts from a provider
  kubectl-mtv get inventory hosts --provider vsphere-prod

  # Output as JSON
  kubectl-mtv get inventory hosts --provider vsphere-prod --output json`,
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

			logNamespaceOperation("Getting hosts from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListHostsWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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

// NewInventoryDataCenterCmd creates the get inventory datacenter command
func NewInventoryDataCenterCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "datacenter",
		Short: "Get datacenters from a provider",
		Long: `Get datacenters from a provider's inventory.

Lists datacenters from vSphere or oVirt providers. Datacenters are the top-level
organizational units that contain clusters, hosts, and VMs.`,
		Example: `  # Filter datacenters by name
  kubectl-mtv get inventory datacenters --provider vsphere-prod --query "where name ~= 'DC.*'"

  # List all datacenters from a provider
  kubectl-mtv get inventory datacenters --provider vsphere-prod

  # Output as YAML
  kubectl-mtv get inventory datacenters --provider vsphere-prod --output yaml`,
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

			logNamespaceOperation("Getting datacenters from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListDataCentersWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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

// NewInventoryClusterCmd creates the get inventory cluster command
func NewInventoryClusterCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Get clusters from a provider",
		Long: `Get clusters from a provider's inventory.

Lists compute clusters from vSphere or oVirt providers. Clusters group hosts
together and define resource pools for VMs.`,
		Example: `  # Filter clusters by datacenter
  kubectl-mtv get inventory clusters --provider vsphere-prod --query "where datacenter = 'DC1'"

  # List all clusters from a provider
  kubectl-mtv get inventory clusters --provider vsphere-prod

  # Output as JSON
  kubectl-mtv get inventory clusters --provider vsphere-prod --output json`,
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

			logNamespaceOperation("Getting clusters from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListClustersWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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

// NewInventoryDiskCmd creates the get inventory disk command
func NewInventoryDiskCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "disk",
		Short: "Get disks from a provider",
		Long: `Get disks from a provider's inventory.

Lists virtual disks from vSphere or oVirt providers. Disk information includes
size, storage location, and attachment to VMs.`,
		Example: `  # Filter disks by size using SI units (greater than 100GB)
  kubectl-mtv get inventory disks --provider vsphere-prod --query "where capacity > 100Gi"

  # List all disks from a provider
  kubectl-mtv get inventory disks --provider ovirt-prod

  # Output as JSON
  kubectl-mtv get inventory disks --provider vsphere-prod --output json`,
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

			logNamespaceOperation("Getting disks from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListDisksWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
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
