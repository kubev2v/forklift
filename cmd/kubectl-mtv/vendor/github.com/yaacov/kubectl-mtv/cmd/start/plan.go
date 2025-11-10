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
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	var cutoverTimeStr string
	var all bool

	cmd := &cobra.Command{
		Use:               "plan [NAME...] [--all]",
		Short:             "Start one or more migration plans",
		Args:              flags.ValidateAllFlagArgs(func() bool { return all }, 1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the global configuration
			config := getGlobalConfig()

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(config.GetKubeConfigFlags())

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
				planNames, err = client.GetAllPlanNames(cmd.Context(), config.GetKubeConfigFlags(), namespace)
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

			// Loop over each plan name and start it
			for _, name := range planNames {
				err := plan.Start(config.GetKubeConfigFlags(), name, namespace, cutoverTime, config.GetUseUTC())
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&cutoverTimeStr, "cutover", "c", "", "Cutover time in ISO8601 format (e.g., 2023-12-31T15:30:00Z, '$(date -d \"+1 hour\" --iso-8601=sec)' ). If not provided, defaults to 1 hour from now.")
	cmd.Flags().BoolVar(&all, "all", false, "Start all migration plans in the namespace")

	return cmd
}
