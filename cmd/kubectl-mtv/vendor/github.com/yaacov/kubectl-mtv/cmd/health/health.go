package health

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	pkghealth "github.com/yaacov/kubectl-mtv/pkg/cmd/health"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// GlobalConfigGetter is an interface for accessing global configuration
type GlobalConfigGetter interface {
	GetAllNamespaces() bool
	GetVerbosity() int
}

// NewHealthCmd creates the health command
func NewHealthCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var skipLogs bool
	var logLines int
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check the health of the MTV/Forklift system",
		Long: `Perform comprehensive health checks on the MTV/Forklift migration system.

This command checks:
- MTV Operator installation and version
- ForkliftController configuration (feature flags, VDDK image, custom images)
- Forklift pod health (status, restarts, OOMKilled)
- Provider connectivity and readiness
- Migration plan status and issues
- Pod logs for errors and warnings (can be skipped with --skip-logs)

Namespace behavior:
  Forklift OPERATOR components (controller, pods, logs) are always checked in
  the auto-detected operator namespace (typically openshift-mtv), regardless
  of the -n flag.

  The -n and -A flags control the scope for USER RESOURCES:
  - Providers: checked in the specified namespace or all namespaces with -A
  - Plans: checked in the specified namespace or all namespaces with -A

  Configuration warnings (e.g., missing VDDK image for vSphere migrations)
  only appear if relevant providers exist in the scoped namespace(s).
  Use -A to check all namespaces cluster-wide.

Examples:
  # Check health in the default MTV namespace (includes log analysis)
  kubectl mtv health

  # Check health with JSON output
  kubectl mtv health -o json

  # Check health for providers/plans in a specific namespace
  kubectl mtv health -n my-namespace

  # Check health across all namespaces (recommended for full cluster check)
  kubectl mtv health -A

  # Check health without log analysis (faster)
  kubectl mtv health --skip-logs

  # Check health with more log lines analyzed
  kubectl mtv health --log-lines 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			// Get namespace from flag or use default
			namespace := ""
			if kubeConfigFlags.Namespace != nil && *kubeConfigFlags.Namespace != "" {
				namespace = *kubeConfigFlags.Namespace
			}

			// Build health check options
			opts := pkghealth.HealthCheckOptions{
				Namespace:     namespace,
				AllNamespaces: globalConfig.GetAllNamespaces(),
				CheckLogs:     !skipLogs,
				LogLines:      logLines,
				Verbose:       globalConfig.GetVerbosity() > 0,
			}

			// Run health check
			report, err := pkghealth.RunHealthCheck(ctx, kubeConfigFlags, opts)
			if err != nil {
				return fmt.Errorf("health check failed: %v", err)
			}

			// Print the report
			return pkghealth.PrintHealthReport(report, outputFormatFlag.GetValue())
		},
	}

	// Add flags
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVar(&skipLogs, "skip-logs", false, "Skip pod log analysis (faster but less thorough)")
	cmd.Flags().IntVar(&logLines, "log-lines", 100, "Number of log lines to analyze per pod")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
