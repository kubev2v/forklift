package archive

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewArchiveCmd creates the archive command with all its subcommands
func NewArchiveCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "archive",
		Short:        "Archive resources",
		Long:         `Archive various MTV resources`,
		SilenceUsage: true,
	}

	// Add plan subcommand with plural alias
	planCmd := NewPlanCmd(kubeConfigFlags)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)
	return cmd
}
