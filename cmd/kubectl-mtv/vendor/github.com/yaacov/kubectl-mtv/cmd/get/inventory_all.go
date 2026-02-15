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

// NewInventoryNetworkCmd creates the get inventory network command
func NewInventoryNetworkCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Get networks from a provider",
		Long: `Get networks from a provider's inventory.

Queries the MTV inventory service to list networks available in the source provider.
Use --query to filter results using TSL query syntax.`,
		Example: `  # Filter networks by name
  kubectl-mtv get inventory networks --provider vsphere-prod --query "where name ~= 'VM Network.*'"

  # List all networks from a provider
  kubectl-mtv get inventory networks --provider vsphere-prod

  # Output as JSON
  kubectl-mtv get inventory networks --provider vsphere-prod --output json`,
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

			logNamespaceOperation("Getting networks from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListNetworksWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider name")
	_ = cmd.MarkFlagRequired("provider")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'VM Network.*'\")")
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

// NewInventoryStorageCmd creates the get inventory storage command
func NewInventoryStorageCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Get storage from a provider",
		Long: `Get storage resources from a provider's inventory.

Queries the MTV inventory service to list storage domains (oVirt), datastores (vSphere),
or storage classes (OpenShift) available in the source provider.`,
		Example: `  # Filter storage by name pattern
  kubectl-mtv get inventory storages --provider ovirt-prod --query "where name ~= 'data.*'"

  # List all storage from a provider
  kubectl-mtv get inventory storages --provider vsphere-prod

  # Output as YAML
  kubectl-mtv get inventory storages --provider vsphere-prod --output yaml`,
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

			logNamespaceOperation("Getting storage from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListStorageWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider name")
	_ = cmd.MarkFlagRequired("provider")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'data.*' and type = 'VMFS'\")")
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

// NewInventoryVMCmd creates the get inventory vm command
func NewInventoryVMCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewVMInventoryOutputTypeFlag()
	var extendedOutput bool
	var query string
	var watch bool
	var provider string

	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Get VMs from a provider",
		Long: `Get virtual machines from a provider's inventory.

Queries the MTV inventory service to list VMs available for migration.
Use --query to filter results using TSL query syntax. The --extended
flag shows additional VM details.

Output format 'planvms' generates YAML suitable for use with 'create plan --vms @file'.

Query Language (TSL):
  Use --query "where ..." to filter inventory results with TSL query syntax:
    --query "where name ~= 'prod-.*'"
    --query "where powerState = 'poweredOn' and memoryMB > 4096"
    --query "where len(disks) > 1 and cpuCount <= 8"
    --query "where any(concerns[*].category = 'Critical')"
    --query "where name like '%web%' order by memoryMB desc limit 10"

  Supports comparison, regex, logical operators, array functions (len(), any(), all()),
  SI units (Ki, Mi, Gi), sorting (ORDER BY), and limiting (LIMIT).

  To discover available fields for your provider, run:
    kubectl-mtv get inventory vm --provider <provider> --output json

  Run 'kubectl-mtv help tsl' for the full syntax reference and field list.`,
		Example: `  # Find VMs with multiple NICs (array length)
  kubectl-mtv get inventory vms --provider vsphere-prod --query "where len(nics) >= 2 and cpuCount > 1"

  # Find VMs with shared disks (any element match)
  kubectl-mtv get inventory vms --provider vsphere-prod --query "where any(disks[*].shared = true)"

  # Find VMs with critical migration concerns
  kubectl-mtv get inventory vms --provider vsphere-prod --query "where any(concerns[*].category = 'Critical')"

  # Filter VMs by name, CPU, and memory
  kubectl-mtv get inventory vms --provider vsphere-prod --query "where name ~= 'web-.*' and memoryMB > 4096"

  # List all VMs from a provider
  kubectl-mtv get inventory vms --provider vsphere-prod

  # Show extended VM details
  kubectl-mtv get inventory vms --provider vsphere-prod --extended

  # Export VMs for plan creation
  kubectl-mtv get inventory vms --provider vsphere-prod --query "where name ~= 'prod-.*'" --output planvms > vms.yaml
  kubectl-mtv create plan --name my-migration --vms @vms.yaml`,
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

			logNamespaceOperation("Getting VMs from provider", namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListVMsWithInsecure(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), extendedOutput, query, watch, inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider name")
	_ = cmd.MarkFlagRequired("provider")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml, planvms)")
	cmd.Flags().BoolVar(&extendedOutput, "extended", false, "Show extended output")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'web-.*' and cpuCount > 4\")")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for provider and output format flags
	if err := cmd.RegisterFlagCompletionFunc("provider", completion.ProviderNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}
	// Custom completion for inventory VM output format that includes planvms
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
