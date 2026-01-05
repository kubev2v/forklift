package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewMappingCmd creates the get mapping command with subcommands
func NewMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mapping",
		Short:        "Get mappings",
		Long:         `Get network and storage mappings`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newGetNetworkMappingCmd(kubeConfigFlags, getGlobalConfig))
	cmd.AddCommand(newGetStorageMappingCmd(kubeConfigFlags, getGlobalConfig))

	return cmd
}

// newGetNetworkMappingCmd creates the get network mapping subcommand
func newGetNetworkMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:          "network [NAME]",
		Short:        "Get network mappings",
		Long:         `Get network mappings`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.MappingNameCompletion(kubeConfigFlags, "network")(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			// Get optional mapping name from arguments
			var mappingName string
			if len(args) > 0 {
				mappingName = args[0]
			}

			// Log the operation being performed
			if mappingName != "" {
				logNamespaceOperation("Getting network mapping", namespace, config.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting network mappings", namespace, config.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return mapping.List(ctx, config.GetKubeConfigFlags(), "network", namespace, outputFormatFlag.GetValue(), mappingName, config.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}

// newGetStorageMappingCmd creates the get storage mapping subcommand
func newGetStorageMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:          "storage [NAME]",
		Short:        "Get storage mappings",
		Long:         `Get storage mappings`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.MappingNameCompletion(kubeConfigFlags, "storage")(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			// Get optional mapping name from arguments
			var mappingName string
			if len(args) > 0 {
				mappingName = args[0]
			}

			// Log the operation being performed
			if mappingName != "" {
				logNamespaceOperation("Getting storage mapping", namespace, config.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting storage mappings", namespace, config.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return mapping.List(ctx, config.GetKubeConfigFlags(), "storage", namespace, outputFormatFlag.GetValue(), mappingName, config.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
