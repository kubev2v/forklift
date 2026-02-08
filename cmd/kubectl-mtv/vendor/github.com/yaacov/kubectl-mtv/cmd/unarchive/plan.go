package unarchive

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/archive/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the plan unarchive command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "plan [NAME...] [--all]",
		Short: "Unarchive one or more migration plans",
		Long: `Unarchive one or more migration plans.

Unarchiving restores a previously archived plan, allowing it to be started again.
This is useful if you need to retry a migration or make changes to an archived plan.`,
		Example: `  # Unarchive a plan
  kubectl-mtv unarchive plan my-migration

  # Unarchive multiple plans
  kubectl-mtv unarchive plan plan1 plan2 plan3

  # Unarchive all archived plans in the namespace
  kubectl-mtv unarchive plan --all`,
		Args:              flags.ValidateAllFlagArgs(func() bool { return all }, 1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			var planNames []string
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
			} else {
				planNames = args
			}

			// Loop over each plan name and unarchive it
			for _, name := range planNames {
				err := plan.Archive(cmd.Context(), kubeConfigFlags, name, namespace, false) // Set archived to false for unarchiving
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Unarchive all migration plans in the namespace")

	return cmd
}
