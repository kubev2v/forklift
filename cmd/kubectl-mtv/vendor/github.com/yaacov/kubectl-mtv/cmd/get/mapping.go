package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewMappingCmd creates the get mapping command with subcommands
func NewMappingCmd(globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watchFlag bool
	var mappingName string

	cmd := &cobra.Command{
		Use:   "mapping",
		Short: "Get mappings",
		Long: `Get network and storage mappings.

When called without a subcommand, lists both network and storage mappings.
Use 'mapping network' or 'mapping storage' subcommands to view a specific
mapping type.`,
		Example: `  # List all mappings (both network and storage)
  kubectl-mtv get mappings

  # Get a specific mapping by name (searches both types)
  kubectl-mtv get mapping --name my-mapping --output yaml

  # Watch all mapping changes
  kubectl-mtv get mappings --watch

  # List only network mappings
  kubectl-mtv get mapping network

  # List only storage mappings
  kubectl-mtv get mapping storage`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watchFlag {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

			// Log the operation being performed
			if mappingName != "" {
				logNamespaceOperation("Getting mapping", namespace, globalConfig.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting all mappings", namespace, globalConfig.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return mapping.List(ctx, globalConfig.GetKubeConfigFlags(), "all", namespace, watchFlag, outputFormatFlag.GetValue(), mappingName, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&mappingName, "name", "M", "", "Mapping name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch for changes")
	help.MarkMCPHidden(cmd, "watch")

	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
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
	var mappingName string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Get network mappings",
		Long: `Get network mappings that define how source provider networks map to target OpenShift networks.

Network mappings translate source VM network connections to target network attachment
definitions (NADs) or pod networking.`,
		Example: `  # List all network mappings
  kubectl-mtv get mapping network

  # Get a specific network mapping in YAML
  kubectl-mtv get mapping network --name my-network-map --output yaml

  # Watch network mapping changes
  kubectl-mtv get mapping network --watch`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

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

	cmd.Flags().StringVarP(&mappingName, "name", "M", "", "Mapping name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")
	help.MarkMCPHidden(cmd, "watch")

	// Add completion for name and output format flags
	if err := cmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.MappingNameCompletion(globalConfig.GetKubeConfigFlags(), "network")(cmd, args, toComplete)
	}); err != nil {
		panic(err)
	}
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
	var mappingName string

	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Get storage mappings",
		Long: `Get storage mappings that define how source provider storage maps to target OpenShift storage classes.

Storage mappings translate source VM datastores/storage domains to target Kubernetes
storage classes with optional volume mode and access mode settings.`,
		Example: `  # List all storage mappings
  kubectl-mtv get mapping storage

  # Get a specific storage mapping in JSON
  kubectl-mtv get mapping storage --name my-storage-map --output json

  # Watch storage mapping changes
  kubectl-mtv get mapping storage --watch`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			namespace := client.ResolveNamespaceWithAllFlag(globalConfig.GetKubeConfigFlags(), globalConfig.GetAllNamespaces())

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

	cmd.Flags().StringVarP(&mappingName, "name", "M", "", "Mapping name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")
	help.MarkMCPHidden(cmd, "watch")

	// Add completion for name and output format flags
	if err := cmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.MappingNameCompletion(globalConfig.GetKubeConfigFlags(), "storage")(cmd, args, toComplete)
	}); err != nil {
		panic(err)
	}
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
