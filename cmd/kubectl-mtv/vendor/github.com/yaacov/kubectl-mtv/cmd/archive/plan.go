package archive

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/archive/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewPlanCmd creates the plan archiving command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var planNames []string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Archive one or more migration plans",
		Long: `Archive one or more migration plans.

Archiving a plan marks it as completed and stops any ongoing operations.
Archived plans are retained for historical reference but cannot be started.
Use 'unarchive' to restore a plan if needed.`,
		Example: `  # Archive a completed plan
  kubectl-mtv archive plan --name my-migration

  # Archive multiple plans
  kubectl-mtv archive plans --name plan1,plan2,plan3

  # Archive all plans in the namespace
  kubectl-mtv archive plans --all`,
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

	cmd.Flags().StringSliceVarP(&planNames, "name", "M", nil, "Plan name(s) to archive (comma-separated, e.g. \"plan1,plan2\")")
	cmd.Flags().StringSliceVar(&planNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")
	cmd.Flags().BoolVar(&all, "all", false, "Archive all migration plans in the namespace")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags))

	return cmd
}
