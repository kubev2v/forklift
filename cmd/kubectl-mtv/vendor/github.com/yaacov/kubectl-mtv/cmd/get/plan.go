package get

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the get plan command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool
	var vms bool

	cmd := &cobra.Command{
		Use:               "plan [NAME]",
		Short:             "Get migration plans",
		Long:              `Get migration plans`,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			config := getGlobalConfig()
			namespace := client.ResolveNamespaceWithAllFlag(config.GetKubeConfigFlags(), config.GetAllNamespaces())

			// Get optional plan name from arguments
			var planName string
			if len(args) > 0 {
				planName = args[0]
			}

			// If --vms flag is used, switch to ListVMs behavior
			if vms {
				if planName == "" {
					return fmt.Errorf("plan NAME is required when using --vms flag")
				}
				// Log the operation being performed
				logNamespaceOperation("Getting plan VMs", namespace, config.GetAllNamespaces())
				logOutputFormat(outputFormatFlag.GetValue())

				return plan.ListVMs(ctx, config.GetKubeConfigFlags(), planName, namespace, watch)
			}

			// Default behavior: list plans

			// Log the operation being performed
			if planName != "" {
				logNamespaceOperation("Getting plan", namespace, config.GetAllNamespaces())
			} else {
				logNamespaceOperation("Getting plans", namespace, config.GetAllNamespaces())
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return plan.List(ctx, config.GetKubeConfigFlags(), namespace, watch, outputFormatFlag.GetValue(), planName, config.GetUseUTC())
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")
	cmd.Flags().BoolVar(&vms, "vms", false, "Get VMs status in the migration plan (requires plan NAME)")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
