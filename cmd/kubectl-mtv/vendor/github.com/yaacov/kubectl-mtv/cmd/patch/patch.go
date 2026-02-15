package patch

import (
	"github.com/spf13/cobra"
	"github.com/yaacov/kubectl-mtv/pkg/util/config"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GlobalConfigGetter is a type alias for the shared config interface.
// This maintains backward compatibility while using the centralized interface definition.
type GlobalConfigGetter = config.InventoryConfigWithKubeFlags

// NewPatchCmd creates the patch command with subcommands
func NewPatchCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "patch",
		Short:        "Patch resources",
		Long:         `Patch various Migration Toolkit for Virtualization resources`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands
	mappingCmd := NewMappingCmd(kubeConfigFlags, globalConfig)
	mappingCmd.Aliases = []string{"mappings"}
	cmd.AddCommand(mappingCmd)

	providerCmd := NewProviderCmd(kubeConfigFlags)
	providerCmd.Aliases = []string{"providers"}
	cmd.AddCommand(providerCmd)

	planCmd := NewPlanCmd(kubeConfigFlags)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)

	planVMCmd := NewPlanVMCmd(kubeConfigFlags)
	planVMCmd.Aliases = []string{"planvms"}
	cmd.AddCommand(planVMCmd)

	return cmd
}
