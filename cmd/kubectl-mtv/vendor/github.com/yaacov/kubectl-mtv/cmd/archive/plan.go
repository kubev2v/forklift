package archive

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/archive/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the plan archiving command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "plan [NAME...] [--all]",
		Short: "Archive one or more migration plans",
		Long: `Archive one or more migration plans.

Archiving a plan marks it as completed and stops any ongoing operations.
Archived plans are retained for historical reference but cannot be started.
Use 'unarchive' to restore a plan if needed.`,
		Example: `  # Archive a completed plan
  kubectl-mtv archive plan my-migration

  # Archive multiple plans
  kubectl-mtv archive plan plan1 plan2 plan3

  # Archive all plans in the namespace
  kubectl-mtv archive plan --all`,
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

			// Loop over each plan name and archive it
			for _, name := range planNames {
				err := plan.Archive(cmd.Context(), kubeConfigFlags, name, namespace, true)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Archive all migration plans in the namespace")

	return cmd
}
