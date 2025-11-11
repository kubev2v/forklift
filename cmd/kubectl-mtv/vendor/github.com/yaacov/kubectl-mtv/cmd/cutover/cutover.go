package cutover

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewCutoverCmd creates the cutover command with all its subcommands
func NewCutoverCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "cutover",
		Short:        "Set cutover time for resources",
		Long:         `Set cutover time for various MTV resources`,
		SilenceUsage: true,
	}

	// Add plan subcommand with plural alias
	planCmd := NewPlanCmd(kubeConfigFlags)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)
	return cmd
}
