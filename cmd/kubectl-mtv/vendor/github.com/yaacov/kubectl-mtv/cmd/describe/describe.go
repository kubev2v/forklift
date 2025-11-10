package describe

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
)

// NewDescribeCmd creates the describe command with all its subcommands
func NewDescribeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "describe",
		Short:        "Describe resources",
		Long:         `Describe migration plans and VMs in migration plans`,
		SilenceUsage: true,
	}

	cmd.AddCommand(NewPlanCmd(kubeConfigFlags, getGlobalConfig))
	cmd.AddCommand(NewHostCmd(kubeConfigFlags, getGlobalConfig))
	cmd.AddCommand(NewHookCmd(kubeConfigFlags, getGlobalConfig))
	cmd.AddCommand(NewMappingCmd(kubeConfigFlags, getGlobalConfig))

	return cmd
}
