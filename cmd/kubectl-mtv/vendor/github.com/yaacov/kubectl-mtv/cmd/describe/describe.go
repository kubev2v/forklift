package describe

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
)

// NewDescribeCmd creates the describe command with all its subcommands
func NewDescribeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "describe",
		Short:        "Describe resources",
		Long:         `Describe migration plans and VMs in migration plans`,
		SilenceUsage: true,
	}

	planCmd := NewPlanCmd(kubeConfigFlags, globalConfig)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)

	hostCmd := NewHostCmd(kubeConfigFlags, globalConfig)
	hostCmd.Aliases = []string{"hosts"}
	cmd.AddCommand(hostCmd)

	hookCmd := NewHookCmd(kubeConfigFlags, globalConfig)
	hookCmd.Aliases = []string{"hooks"}
	cmd.AddCommand(hookCmd)

	mappingCmd := NewMappingCmd(globalConfig)
	mappingCmd.Aliases = []string{"mappings"}
	cmd.AddCommand(mappingCmd)

	return cmd
}
