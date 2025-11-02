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
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() get.GlobalConfigGetter) *cobra.Command {
	var withVMs bool
	var vmName string
	var watch bool

	cmd := &cobra.Command{
		Use:               "plan NAME",
		Short:             "Describe a migration plan",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Get the global configuration
			config := getGlobalConfig()

			// Validate that --with-vms and --vm are mutually exclusive
			if withVMs && vmName != "" {
				return fmt.Errorf("--with-vms and --vm flags are mutually exclusive")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(config.GetKubeConfigFlags())

			// If --vm flag is provided, switch to VM description behavior
			if vmName != "" {
				return vm.DescribeVM(config.GetKubeConfigFlags(), name, namespace, vmName, watch, config.GetUseUTC())
			}

			// Default behavior: describe plan
			return plan.Describe(config.GetKubeConfigFlags(), name, namespace, withVMs, config.GetUseUTC())
		},
	}

	cmd.Flags().BoolVar(&withVMs, "with-vms", false, "Include list of VMs in the plan specification")
	cmd.Flags().StringVar(&vmName, "vm", "", "VM name to describe (switches to VM description mode)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch VM status with live updates (only when --vm is used)")

	return cmd
}
