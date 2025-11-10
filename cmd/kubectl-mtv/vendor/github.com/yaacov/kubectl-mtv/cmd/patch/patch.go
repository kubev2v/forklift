package patch

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewPatchCmd creates the patch command with subcommands
func NewPatchCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
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
	cmd.AddCommand(NewMappingCmd(kubeConfigFlags))
	cmd.AddCommand(NewProviderCmd(kubeConfigFlags))
	cmd.AddCommand(NewPlanCmd(kubeConfigFlags))
	cmd.AddCommand(NewPlanVMCmd(kubeConfigFlags))

	return cmd
}
