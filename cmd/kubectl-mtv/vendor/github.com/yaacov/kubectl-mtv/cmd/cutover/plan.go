package cutover

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/cutover/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewPlanCmd creates the plan cutover command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var cutoverTimeStr string
	var all bool
	var planNames []string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Set the cutover time for one or more warm migration plans",
		Long: `Trigger cutover for warm migration plans.

Cutover stops the source VMs and performs the final sync to complete the migration.
Use this to manually trigger cutover for warm migrations, or to reschedule
a cutover time. If no cutover time is specified, it defaults to immediately.`,
		Example: `  # Trigger immediate cutover
  kubectl-mtv cutover plan --name my-warm-migration

  # Schedule cutover for a specific time
  kubectl-mtv cutover plan --name my-warm-migration --cutover 2026-12-31T23:00:00Z

  # Cutover all warm migration plans
  kubectl-mtv cutover plans --all

  # Cutover multiple plans
  kubectl-mtv cutover plans --name plan1,plan2,plan3`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate mutual exclusivity of --name and --all
			if all && len(planNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(planNames) == 0 {
				return errors.New("must specify --name or --all")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			var cutoverTime *time.Time
			if cutoverTimeStr != "" {
				// Parse the provided cutover time
				t, err := time.Parse(time.RFC3339, cutoverTimeStr)
				if err != nil {
					return fmt.Errorf("failed to parse cutover time: %v", err)
				}
				cutoverTime = &t
			}

			if all {
				// Get all plan names from the namespace
				var err error
				planNames, err = client.GetAllPlanNames(cmd.Context(), kubeConfigFlags, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all plan names: %v", err)
				}
				if len(planNames) == 0 {
					fmt.Printf("No plans found in namespace %s\n", namespace)
					return nil
				}
			}

			// Loop over each plan name and set cutover time
			for _, planName := range planNames {
				err := plan.Cutover(kubeConfigFlags, planName, namespace, cutoverTime)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&planNames, "name", "M", nil, "Plan name(s) to cutover (comma-separated, e.g. \"plan1,plan2\")")
	cmd.Flags().StringSliceVar(&planNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")
	cmd.Flags().StringVarP(&cutoverTimeStr, "cutover", "c", "", "Cutover time in ISO8601 format (e.g., 2023-12-31T15:30:00Z, '$(date --iso-8601=sec)'). If not specified, defaults to current time.")
	cmd.Flags().BoolVar(&all, "all", false, "Set cutover time for all migration plans in the namespace")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags))

	return cmd
}
