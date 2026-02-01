package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewMappingCmd creates the get mapping command with subcommands
func NewMappingCmd(globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mapping",
		Short: "Get mappings",
		Long: `Get network and storage mappings.

Mappings define how source provider resources (networks, storage) are translated
to target OpenShift resources during migration. Use 'mapping network' or
'mapping storage' subcommands to view specific mapping types.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newGetNetworkMappingCmd(globalConfig))
	cmd.AddCommand(newGetStorageMappingCmd(globalConfig))

	return cmd
}

// newGetNetworkMappingCmd creates the get network mapping subcommand
func newGetNetworkMappingCmd(globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool

	cmd := &cobra.Command{
		Use:   "network [NAME]",
		Short: "Get network mappings",
		Long: `Get network mappings that define how source provider networks map to target OpenShift networks.

Network mappings translate source VM network connections to target network attachment
definitions (NADs) or pod networking.`,
		Example: `  # List all network mappings
  kubectl-mtv get mapping network

  # Get a specific network mapping in YAML
  kubectl-mtv get mapping network my-network-map -o yaml

  # Watch network mapping changes
  kubectl-mtv get mapping network -w`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.MappingNameCompletion(globalConfig.GetKubeConfigFlags(), "network")(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			// Get optional mapping name from arguments
			var mappingName string
			if len(args) > 0 {
				mappingName = args[0]
			}

			// Log the operation being performed
			if mappingName != "" {
				logNamespaceOperation("Getting network mapping", namespace, globalConfig.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting network mappings", namespace, globalConfig.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return mapping.List(ctx, globalConfig.GetKubeConfigFlags(), "network", namespace, watch, outputFormatFlag.GetValue(), mappingName, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}

// newGetStorageMappingCmd creates the get storage mapping subcommand
func newGetStorageMappingCmd(globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool

	cmd := &cobra.Command{
		Use:   "storage [NAME]",
		Short: "Get storage mappings",
		Long: `Get storage mappings that define how source provider storage maps to target OpenShift storage classes.

Storage mappings translate source VM datastores/storage domains to target Kubernetes
storage classes with optional volume mode and access mode settings.`,
		Example: `  # List all storage mappings
  kubectl-mtv get mapping storage

  # Get a specific storage mapping in JSON
  kubectl-mtv get mapping storage my-storage-map -o json

  # Watch storage mapping changes
  kubectl-mtv get mapping storage -w`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.MappingNameCompletion(globalConfig.GetKubeConfigFlags(), "storage")(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			// Get optional mapping name from arguments
			var mappingName string
			if len(args) > 0 {
				mappingName = args[0]
			}

			// Log the operation being performed
			if mappingName != "" {
				logNamespaceOperation("Getting storage mapping", namespace, globalConfig.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting storage mappings", namespace, globalConfig.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return mapping.List(ctx, globalConfig.GetKubeConfigFlags(), "storage", namespace, watch, outputFormatFlag.GetValue(), mappingName, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
