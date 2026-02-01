package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

const defaultInventoryTimeout = 280 * time.Second

// ec2CommandConfig holds the configuration for creating an EC2 inventory command
type ec2CommandConfig struct {
	use        string
	short      string
	long       string
	logMessage string
	listFunc   func(ctx context.Context, flags *genericclioptions.ConfigFlags, provider, namespace, inventoryURL, outputFormat, query string, watch, insecure bool) error
}

// newEC2InventoryCmd creates a new EC2 inventory command with the given configuration
func newEC2InventoryCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter, cfg ec2CommandConfig) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string
	var watch bool

	cmd := &cobra.Command{
		Use:               cfg.use,
		Short:             cfg.short,
		Long:              cfg.long,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, defaultInventoryTimeout)
				defer cancel()
			}

			provider := args[0]

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			logNamespaceOperation(cfg.logMessage, namespace, globalConfig.GetAllNamespaces())
			logOutputFormat(outputFormatFlag.GetValue())

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return cfg.listFunc(ctx, globalConfig.GetKubeConfigFlags(), provider, namespace, inventoryURL, outputFormatFlag.GetValue(), query, watch, inventoryInsecureSkipTLS)
		},
	}
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		klog.V(2).Infof("Failed to register output flag completion: %v", err)
	}

	return cmd
}

// NewInventoryEC2InstanceCmd creates the get inventory instance command for EC2
func NewInventoryEC2InstanceCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newEC2InventoryCmd(kubeConfigFlags, globalConfig, ec2CommandConfig{
		use:        "ec2-instance PROVIDER",
		short:      "Get EC2 instances from a provider " + flags.ProvidersEC2,
		long:       `Get EC2 instances from an AWS provider's inventory.`,
		logMessage: "Getting EC2 instances from provider",
		listFunc:   inventory.ListEC2InstancesWithInsecure,
	})
}

// NewInventoryEC2VolumeCmd creates the get inventory volume command for EC2 EBS volumes
func NewInventoryEC2VolumeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newEC2InventoryCmd(kubeConfigFlags, globalConfig, ec2CommandConfig{
		use:        "ec2-volume PROVIDER",
		short:      "Get EC2 EBS volumes from a provider " + flags.ProvidersEC2,
		long:       `Get EC2 EBS volumes (disks) from an AWS provider's inventory.`,
		logMessage: "Getting EC2 EBS volumes from provider",
		listFunc:   inventory.ListEC2VolumesWithInsecure,
	})
}

// NewInventoryEC2VolumeTypeCmd creates the get inventory volume-type command for EC2 storage classes
func NewInventoryEC2VolumeTypeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newEC2InventoryCmd(kubeConfigFlags, globalConfig, ec2CommandConfig{
		use:        "ec2-volume-type PROVIDER",
		short:      "Get EC2 EBS volume types from a provider " + flags.ProvidersEC2,
		long:       `Get EC2 EBS volume types (storage classes like gp3, io2, etc.) from an AWS provider's inventory.`,
		logMessage: "Getting EC2 volume types from provider",
		listFunc:   inventory.ListEC2VolumeTypesWithInsecure,
	})
}

// NewInventoryEC2NetworkCmd creates the get inventory network command for EC2 (VPCs and Subnets)
func NewInventoryEC2NetworkCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newEC2InventoryCmd(kubeConfigFlags, globalConfig, ec2CommandConfig{
		use:        "ec2-network PROVIDER",
		short:      "Get EC2 networks (VPCs and Subnets) from a provider " + flags.ProvidersEC2,
		long:       `Get EC2 networks (VPCs and Subnets) from an AWS provider's inventory.`,
		logMessage: "Getting EC2 networks from provider",
		listFunc:   inventory.ListEC2NetworksWithInsecure,
	})
}
