package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/host"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHostCmd creates the host description command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "host",
		Short: "Describe a migration host",
		Long: `Display detailed information about a migration host.

Shows host configuration, IP address, provider reference, and status conditions.`,
		Example: `  # Describe a host
  kubectl-mtv describe host --name esxi-host-1`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.ResolveNameArg(&name, args); err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()
			return host.Describe(cmd.Context(), globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC(), inventoryInsecureSkipTLS, outputFormatFlag.GetValue())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Host name")
	flags.MarkRequiredForMCP(cmd, "name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HostResourceNameCompletion(kubeConfigFlags))
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}
