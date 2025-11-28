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
	cmd.AddCommand(NewMappingCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(NewProviderCmd(kubeConfigFlags))
	cmd.AddCommand(NewPlanCmd(kubeConfigFlags))
	cmd.AddCommand(NewPlanVMCmd(kubeConfigFlags))

	return cmd
}
