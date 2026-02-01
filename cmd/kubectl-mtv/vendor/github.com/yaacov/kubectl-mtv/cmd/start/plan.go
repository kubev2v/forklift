package start

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/start/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the plan start command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var cutoverTimeStr string
	var all bool

	cmd := &cobra.Command{
		Use:   "plan [NAME...] [--all]",
		Short: "Start one or more migration plans",
		Long: `Start one or more migration plans.

For cold migrations, the migration begins immediately. For warm migrations,
you can optionally specify a cutover time; if not provided, cutover defaults
to 1 hour from the start time.

The plan must be in a 'Ready' state to be started.`,
		Example: `  # Start a migration plan
  kubectl-mtv start plan my-migration

  # Start multiple plans
  kubectl-mtv start plan plan1 plan2 plan3

  # Start all plans in the namespace
  kubectl-mtv start plan --all

  # Start with scheduled cutover (warm migration)
  kubectl-mtv start plan my-migration --cutover 2026-12-31T23:00:00Z

  # Start warm migration with cutover in 2 hours (Linux)
  kubectl-mtv start plan my-migration --cutover "$(date -d '+2 hours' --iso-8601=sec)"`,
		Args:              flags.ValidateAllFlagArgs(func() bool { return all }, 1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Cache kubeconfig flags for reuse throughout the function
			cfg := globalConfig.GetKubeConfigFlags()

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(cfg)

			var cutoverTime *time.Time
			if cutoverTimeStr != "" {
				// Parse the provided cutover time
				t, err := time.Parse(time.RFC3339, cutoverTimeStr)
				if err != nil {
					return fmt.Errorf("failed to parse cutover time: %v", err)
				}
				cutoverTime = &t
			}

			var planNames []string
			if all {
				// Get all plan names from the namespace
				var err error
				planNames, err = client.GetAllPlanNames(cmd.Context(), cfg, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all plan names: %v", err)
				}
				if len(planNames) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No plans found in namespace %s\n", namespace)
					return nil
				}
			} else {
				planNames = args
			}

			// Loop over each plan name and start it
			for _, name := range planNames {
				if err := plan.Start(cfg, name, namespace, cutoverTime, globalConfig.GetUseUTC()); err != nil {
					return fmt.Errorf("failed to start plan %q: %w", name, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&cutoverTimeStr, "cutover", "c", "", "Cutover time in ISO8601 format (e.g., 2023-12-31T15:30:00Z, '$(date -d \"+1 hour\" --iso-8601=sec)' ). If not provided, defaults to 1 hour from now.")
	cmd.Flags().BoolVar(&all, "all", false, "Start all migration plans in the namespace")

	return cmd
}
