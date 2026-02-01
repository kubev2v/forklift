package cancel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/cancel/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewPlanCmd creates the plan cancellation command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var vmNamesOrFile string

	cmd := &cobra.Command{
		Use:   "plan NAME",
		Short: "Cancel specific VMs in a running migration plan",
		Long: `Cancel specific VMs in a running migration plan.

This command allows you to stop the migration of selected VMs while allowing
other VMs in the plan to continue. VMs to cancel can be specified as a
comma-separated list or read from a file.`,
		Example: `  # Cancel specific VMs in a plan
  kubectl-mtv cancel plan my-migration --vms "vm1,vm2"

  # Cancel VMs from a file
  kubectl-mtv cancel plan my-migration --vms @failed-vms.yaml`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.PlanNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get plan name from positional argument
			planName := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			var vmNames []string

			if strings.HasPrefix(vmNamesOrFile, "@") {
				// It's a file
				filePath := vmNamesOrFile[1:]
				content, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %v", filePath, err)
				}

				// Try to unmarshal as JSON or YAML array of strings
				var namesArray []string
				if err := json.Unmarshal(content, &namesArray); err != nil {
					if err := yaml.Unmarshal(content, &namesArray); err != nil {
						return fmt.Errorf("failed to parse VM names from file: %v", err)
					}
				}
				vmNames = namesArray
			} else {
				// It's a comma-separated list
				vmNameSlice := strings.Split(vmNamesOrFile, ",")
				for _, vmName := range vmNameSlice {
					vmNames = append(vmNames, strings.TrimSpace(vmName))
				}
			}

			if len(vmNames) == 0 {
				return fmt.Errorf("no VM names specified to cancel")
			}

			return plan.Cancel(kubeConfigFlags, planName, namespace, vmNames)
		},
	}

	cmd.Flags().StringVar(&vmNamesOrFile, "vms", "", "List of VM names to cancel (comma-separated) or path to file containing VM names (prefix with @)")

	if err := cmd.MarkFlagRequired("vms"); err != nil {
		fmt.Printf("Warning: error marking 'vms' flag as required: %v\n", err)
	}

	return cmd
}
