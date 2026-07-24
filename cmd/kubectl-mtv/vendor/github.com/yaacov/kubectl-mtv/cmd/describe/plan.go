package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	plan "github.com/yaacov/kubectl-mtv/pkg/cmd/describe/plan"
	vm "github.com/yaacov/kubectl-mtv/pkg/cmd/describe/vm"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the plan description command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string
	var withVMs bool
	var vmName string
	var watch bool
	var withDiagnostics bool
	var logLines int
	var showLines int
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Describe a migration plan",
		Long: `Display detailed information about a migration plan.

Shows plan configuration, status, conditions, and optionally the list of VMs.
Use --vm to see detailed status of a specific VM in the plan.
Use --diagnostics to include pod logs, events, and configuration context.`,
		Example: `  # Describe a plan
  kubectl-mtv describe plan --name my-migration

  # Describe a plan including VM list
  kubectl-mtv describe plan --name my-migration --with-vms

  # Describe a specific VM in the plan
  kubectl-mtv describe plan --name my-migration --vm web-server

  # Watch VM status with live updates
  kubectl-mtv describe plan --name my-migration --vm web-server --watch

  # Include diagnostics (pod logs, events, config)
  kubectl-mtv describe plan --name my-migration --diagnostics

  # Show more log lines in diagnostics
  kubectl-mtv describe plan --name my-migration --diagnostics --show-log-lines 20`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.ResolveNameArg(&name, args); err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Get the global configuration

			// Validate mutual exclusivity
			if withVMs && vmName != "" {
				return fmt.Errorf("--with-vms and --vm flags are mutually exclusive")
			}
			if withDiagnostics && vmName != "" {
				return fmt.Errorf("--diagnostics and --vm flags are mutually exclusive")
			}

			outputFormat := outputFormatFlag.GetValue()

			// --watch only works with table output
			if watch && outputFormat != "table" {
				return fmt.Errorf("--watch and --output %s are mutually exclusive; --watch only works with table output", outputFormat)
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())

			// If --vm flag is provided, switch to VM description behavior
			if vmName != "" {
				return vm.DescribeVM(globalConfig.GetKubeConfigFlags(), name, namespace, vmName, watch, globalConfig.GetUseUTC(), outputFormat)
			}

			// Default behavior: describe plan
			return plan.Describe(globalConfig.GetKubeConfigFlags(), name, namespace, withVMs, withDiagnostics, logLines, showLines, globalConfig.GetUseUTC(), outputFormat)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Plan name")
	flags.MarkRequiredForMCP(cmd, "name")
	cmd.Flags().BoolVar(&withVMs, "with-vms", false, "Include list of VMs in the plan specification")
	cmd.Flags().StringVar(&vmName, "vm", "", "VM name to describe (switches to VM description mode)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch VM status with live updates (only when --vm is used)")
	cmd.Flags().BoolVarP(&withDiagnostics, "diagnostics", "D", false, "Include diagnostics (pod logs, events, configuration context)")
	cmd.Flags().IntVar(&logLines, "scan-log-lines", 500, "Number of log lines to scan for diagnostics (max 10000)")
	cmd.Flags().IntVar(&showLines, "show-log-lines", 10, "Number of log lines to display in diagnostics output (max 500)")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)

	_ = cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags))
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}
