package describe

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewMappingCmd creates the mapping description command with subcommands
func NewMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mapping",
		Short:        "Describe mappings",
		Long:         `Describe network and storage mappings`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newDescribeNetworkMappingCmd(kubeConfigFlags, getGlobalConfig))
	cmd.AddCommand(newDescribeStorageMappingCmd(kubeConfigFlags, getGlobalConfig))

	return cmd
}

// newDescribeNetworkMappingCmd creates the describe network mapping subcommand
func newDescribeNetworkMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "network NAME",
		Short:             "Describe a network mapping",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.MappingNameCompletion(kubeConfigFlags, "network"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Get the global configuration
			config := getGlobalConfig()

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(config.GetKubeConfigFlags())
			return mapping.Describe(config.GetKubeConfigFlags(), "network", name, namespace, config.GetUseUTC())
		},
	}

	return cmd
}

// newDescribeStorageMappingCmd creates the describe storage mapping subcommand
func newDescribeStorageMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "storage NAME",
		Short:             "Describe a storage mapping",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.MappingNameCompletion(kubeConfigFlags, "storage"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Get the global configuration
			config := getGlobalConfig()

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(config.GetKubeConfigFlags())
			return mapping.Describe(config.GetKubeConfigFlags(), "storage", name, namespace, config.GetUseUTC())
		},
	}

	return cmd
}
