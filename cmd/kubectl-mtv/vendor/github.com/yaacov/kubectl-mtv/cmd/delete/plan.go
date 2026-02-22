package delete

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewPlanCmd creates the plan deletion command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var skipArchive bool
	var cleanAll bool
	var planNames []string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Delete one or more migration plans",
		Long: `Delete one or more migration plans.

By default, plans are archived before deletion to preserve history. Use
--skip-archive to delete immediately without archiving. Use --clean-all
to also clean up any target VMs created from failed migrations.`,
		Example: `  # Delete a plan (archives first)
  kubectl-mtv delete plan --name my-migration

  # Delete immediately without archiving
  kubectl-mtv delete plan --name my-migration --skip-archive

  # Delete plan and clean up failed migration VMs
  kubectl-mtv delete plan --name my-migration --clean-all

  # Delete multiple plans
  kubectl-mtv delete plans --name plan1,plan2,plan3

  # Delete all plans in namespace
  kubectl-mtv delete plans --all`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --all and --name are mutually exclusive
			if all && len(planNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(planNames) == 0 {
				return errors.New("either --name or --all is required")
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

			// Loop over each plan name and delete it
			for _, name := range planNames {
				err := plan.Delete(cmd.Context(), kubeConfigFlags, name, namespace, skipArchive, cleanAll)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all migration plans in the namespace")
	cmd.Flags().StringSliceVarP(&planNames, "name", "M", nil, "Plan name(s) to delete (comma-separated, e.g. \"plan1,plan2\")")
	cmd.Flags().StringSliceVar(&planNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")
	cmd.Flags().BoolVar(&skipArchive, "skip-archive", false, "Skip archiving and delete the plan immediately")
	cmd.Flags().BoolVar(&cleanAll, "clean-all", false, "Archive, delete VMs on failed migration, then delete")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags))

	return cmd
}
