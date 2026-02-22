package start

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/start/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewPlanCmd creates the plan start command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var cutoverTimeStr string
	var all bool
	var dryRun bool
	var outputFormat string
	var planNames []string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Start one or more migration plans",
		Long: `Start one or more migration plans.

For cold migrations, the migration begins immediately. For warm migrations,
you can optionally specify a cutover time; if not provided, cutover defaults
to 1 hour from the start time.

The plan must be in a 'Ready' state to be started.

Use --dry-run to output the Migration CR(s) to stdout instead of creating
them in Kubernetes. This is useful for debugging, validation, and inspection.`,
		Example: `  # Start a migration plan
  kubectl-mtv start plan --name my-migration

  # Start multiple plans
  kubectl-mtv start plans --name plan1,plan2,plan3

  # Start all plans in the namespace
  kubectl-mtv start plans --all

  # Start with scheduled cutover (warm migration)
  kubectl-mtv start plan --name my-migration --cutover 2026-12-31T23:00:00Z

  # Start warm migration with cutover in 2 hours (Linux)
  kubectl-mtv start plan --name my-migration --cutover "$(date -d '+2 hours' --iso-8601=sec)"

  # Dry-run: output Migration CR to stdout (YAML format)
  kubectl-mtv start plan --name my-migration --dry-run

  # Dry-run: output Migration CR in JSON format
  kubectl-mtv start plan --name my-migration --dry-run --output json

  # Dry-run: output all Migration CRs in namespace
  kubectl-mtv start plans --all --dry-run`,
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
			}

			// Validate that --output is only used with --dry-run
			if !dryRun && outputFormat != "" {
				return fmt.Errorf("--output flag can only be used with --dry-run")
			}

			// Validate output format for dry-run
			if dryRun && outputFormat != "" && outputFormat != "json" && outputFormat != "yaml" {
				return fmt.Errorf("invalid output format for dry-run: %s. Valid formats are: json, yaml", outputFormat)
			}

			// Set default output format for dry-run
			if dryRun && outputFormat == "" {
				outputFormat = "yaml"
			}

			// Loop over each plan name and start it (dry-run is handled inside plan.Start)
			for _, name := range planNames {
				if err := plan.Start(cfg, name, namespace, cutoverTime, globalConfig.GetUseUTC(), dryRun, outputFormat); err != nil {
					return fmt.Errorf("failed to start plan %q: %w", name, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&planNames, "name", "M", nil, "Plan name(s) to start (comma-separated, e.g. \"plan1,plan2\")")
	cmd.Flags().StringSliceVar(&planNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")
	cmd.Flags().StringVarP(&cutoverTimeStr, "cutover", "c", "", "Cutover time in ISO8601 format (e.g., 2023-12-31T15:30:00Z, '$(date -d \"+1 hour\" --iso-8601=sec)' ). If not provided, defaults to 1 hour from now.")
	cmd.Flags().BoolVar(&all, "all", false, "Start all migration plans in the namespace")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Output Migration CR(s) to stdout instead of creating them")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format for dry-run (json, yaml). Defaults to yaml when --dry-run is used")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags))

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		// Ignore completion registration errors - not critical
		_ = err
	}

	return cmd
}
