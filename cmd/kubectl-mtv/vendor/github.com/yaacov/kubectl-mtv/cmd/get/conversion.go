package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/conversion"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewConversionCmd creates the get conversion command
func NewConversionCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool
	var query string
	var convName string

	cmd := &cobra.Command{
		Use:   "conversion",
		Short: "Get conversions",
		Long: `Get MTV Conversion resources from the cluster.

Conversion resources are created by the plan controller when feature_use_conversion_cr
is enabled. They track the lifecycle of individual VM disk conversions including
inspection, in-place, and remote virt-v2v operations.`,
		Example: `  # List all conversions
  kubectl-mtv get conversions

  # Get a specific conversion
  kubectl-mtv get conversion --name my-conv-abc123

  # Filter conversions by phase
  kubectl-mtv get conversions --query "where phase = 'Running'"`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.ResolveNameArg(&convName, args); err != nil {
				return err
			}

			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			kubeConfigFlags := globalConfig.GetKubeConfigFlags()
			allNamespaces := globalConfig.GetAllNamespaces()
			namespace := client.ResolveNamespaceWithAllFlag(kubeConfigFlags, allNamespaces)

			if convName != "" {
				logNamespaceOperation("Getting conversion", namespace, allNamespaces)
			} else {
				logNamespaceOperation("Getting conversions", namespace, allNamespaces)
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return conversion.List(ctx, kubeConfigFlags, namespace, watch, outputFormatFlag.GetValue(), convName, globalConfig.GetUseUTC(), query)
		},
	}

	cmd.Flags().StringVarP(&convName, "name", "M", "", "Conversion name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where phase = 'Running'\")")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")
	help.MarkMCPHidden(cmd, "watch")

	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}
