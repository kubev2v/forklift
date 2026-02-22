package describe

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewMappingCmd creates the mapping description command with subcommands
func NewMappingCmd(globalConfig get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mapping",
		Short: "Describe mappings",
		Long: `Describe network and storage mappings.

Shows detailed configuration of mappings including source/target pairs,
provider references, and status conditions.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newDescribeNetworkMappingCmd(globalConfig))
	cmd.AddCommand(newDescribeStorageMappingCmd(globalConfig))

	return cmd
}

// newDescribeNetworkMappingCmd creates the describe network mapping subcommand
func newDescribeNetworkMappingCmd(globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Describe a network mapping",
		Long: `Display detailed information about a network mapping.

Shows the source and target network pairs, provider references, and status.`,
		Example: `  # Describe a network mapping
  kubectl-mtv describe mapping network --name my-net-map`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required --name flag
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			return mapping.Describe(globalConfig.GetKubeConfigFlags(), "network", name, namespace, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Network mapping name")
	_ = cmd.MarkFlagRequired("name")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.MappingNameCompletion(globalConfig.GetKubeConfigFlags(), "network"))

	return cmd
}

// newDescribeStorageMappingCmd creates the describe storage mapping subcommand
func newDescribeStorageMappingCmd(globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Describe a storage mapping",
		Long: `Display detailed information about a storage mapping.

Shows the source and target storage pairs, volume modes, access modes, and status.`,
		Example: `  # Describe a storage mapping
  kubectl-mtv describe mapping storage --name my-storage-map`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required --name flag
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			return mapping.Describe(globalConfig.GetKubeConfigFlags(), "storage", name, namespace, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Storage mapping name")
	_ = cmd.MarkFlagRequired("name")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.MappingNameCompletion(globalConfig.GetKubeConfigFlags(), "storage"))

	return cmd
}
