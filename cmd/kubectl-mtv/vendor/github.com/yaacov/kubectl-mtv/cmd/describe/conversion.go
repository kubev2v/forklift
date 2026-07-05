package describe

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/conversion"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewConversionCmd creates the conversion description command
func NewConversionCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "conversion",
		Short: "Describe a conversion",
		Long: `Display detailed information about a Conversion resource.

Shows conversion spec (type, VM, disks, image), status (phase, stage, message),
snapshot state, inspection results, and conditions.`,
		Example: `  # Describe a conversion
  kubectl-mtv describe conversion --name my-conv-abc123`,
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
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return conversion.Describe(ctx, globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC(), outputFormatFlag.GetValue())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Conversion name")
	flags.MarkRequiredForMCP(cmd, "name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)

	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}
