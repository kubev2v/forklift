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
)

// NewPlanCmd creates the plan description command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string
	var withVMs bool
	var vmName string
	var watch bool

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Describe a migration plan",
		Long: `Display detailed information about a migration plan.

Shows plan configuration, status, conditions, and optionally the list of VMs.
Use --vm to see detailed status of a specific VM in the plan.`,
		Example: `  # Describe a plan
  kubectl-mtv describe plan --name my-migration

  # Describe a plan including VM list
  kubectl-mtv describe plan --name my-migration --with-vms

  # Describe a specific VM in the plan
  kubectl-mtv describe plan --name my-migration --vm web-server

  # Watch VM status with live updates
  kubectl-mtv describe plan --name my-migration --vm web-server --watch`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required --name flag
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Get the global configuration

			// Validate that --with-vms and --vm are mutually exclusive
			if withVMs && vmName != "" {
				return fmt.Errorf("--with-vms and --vm flags are mutually exclusive")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())

			// If --vm flag is provided, switch to VM description behavior
			if vmName != "" {
				return vm.DescribeVM(globalConfig.GetKubeConfigFlags(), name, namespace, vmName, watch, globalConfig.GetUseUTC())
			}

			// Default behavior: describe plan
			return plan.Describe(globalConfig.GetKubeConfigFlags(), name, namespace, withVMs, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Plan name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().BoolVar(&withVMs, "with-vms", false, "Include list of VMs in the plan specification")
	cmd.Flags().StringVar(&vmName, "vm", "", "VM name to describe (switches to VM description mode)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch VM status with live updates (only when --vm is used)")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags))

	return cmd
}
