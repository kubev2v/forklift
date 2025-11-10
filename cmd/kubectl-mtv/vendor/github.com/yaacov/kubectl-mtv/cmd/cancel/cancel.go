package cancel

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewCancelCmd creates the cancel command with all its subcommands
func NewCancelCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "cancel",
		Short:        "Cancel resources",
		Long:         `Cancel various MTV resources`,
		SilenceUsage: true,
	}

	cmd.AddCommand(NewPlanCmd(kubeConfigFlags))
	return cmd
}
