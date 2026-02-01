package start

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
)

// NewStartCmd creates the start command with all its subcommands
func NewStartCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start resources",
		Long:         `Start various MTV resources`,
		SilenceUsage: true,
	}

	// Add plan subcommand with plural alias
	planCmd := NewPlanCmd(kubeConfigFlags, globalConfig)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)
	return cmd
}
