package get

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewPlanCmd creates the get plan command
func NewPlanCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var watch bool
	var vms bool
	var disk bool
	var vmsTable bool
	var query string

	var planName string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Get migration plans",
		Long: `Get migration plans from the cluster.

Lists all plans in the namespace, or retrieves details for a specific plan.
Use --vms to see the migration status of individual VMs within a plan.
Use --disk to see the disk transfer status with individual disk details.
Use both --vms and --disk together to see VMs with their disk details.
Use --vms-table to see all VMs across plans in a flat table with source/target inventory details.
Use --query with --vms-table to filter, sort, or select columns using TSL syntax.`,
		Example: `  # List all plans in current namespace
  kubectl-mtv get plans

  # List plans across all namespaces
  kubectl-mtv get plans --all-namespaces

  # Get a specific plan in JSON format
  kubectl-mtv get plan --name my-migration --output json

  # Watch plan status changes
  kubectl-mtv get plan --name my-migration --watch

  # Get VM migration status within a plan
  kubectl-mtv get plan --name my-migration --vms

  # Get disk transfer status within a plan
  kubectl-mtv get plan --name my-migration --disk

  # Get both VM and disk transfer status
  kubectl-mtv get plan --name my-migration --vms --disk

  # Show all VMs across all plans in a table
  kubectl-mtv get plans --vms-table

  # Show VMs for a specific plan in a table
  kubectl-mtv get plan --name my-migration --vms-table

  # Filter VMs table by plan status
  kubectl-mtv get plans --vms-table --query "where planStatus = 'Failed'"

  # Export VMs table as JSON
  kubectl-mtv get plans --vms-table --output json`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !watch {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			kubeConfigFlags := globalConfig.GetKubeConfigFlags()
			allNamespaces := globalConfig.GetAllNamespaces()
			namespace := client.ResolveNamespaceWithAllFlag(kubeConfigFlags, allNamespaces)

			// If --vms-table flag is used, show flat VM table with inventory details
			if vmsTable {
				logNamespaceOperation("Getting VMs table", namespace, allNamespaces)
				logOutputFormat(outputFormatFlag.GetValue())

				inventoryURL := globalConfig.GetInventoryURL()
				inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

				return plan.ListVMsTable(ctx, kubeConfigFlags, planName, namespace, inventoryURL, inventoryInsecureSkipTLS, outputFormatFlag.GetValue(), query, watch)
			}

			// If both --vms and --disk flags are used, show combined view
			if vms && disk {
				if planName == "" {
					return fmt.Errorf("plan NAME is required when using --vms and --disk flags")
				}
				// Log the operation being performed
				logNamespaceOperation("Getting plan VMs with disk details", namespace, allNamespaces)
				logOutputFormat(outputFormatFlag.GetValue())

				return plan.ListVMsWithDisks(ctx, kubeConfigFlags, planName, namespace, watch)
			}

			// If --vms flag is used, switch to ListVMs behavior
			if vms {
				if planName == "" {
					return fmt.Errorf("plan NAME is required when using --vms flag")
				}
				// Log the operation being performed
				logNamespaceOperation("Getting plan VMs", namespace, allNamespaces)
				logOutputFormat(outputFormatFlag.GetValue())

				return plan.ListVMs(ctx, kubeConfigFlags, planName, namespace, watch)
			}

			// If --disk flag is used, switch to ListDisks behavior
			if disk {
				if planName == "" {
					return fmt.Errorf("plan NAME is required when using --disk flag")
				}
				// Log the operation being performed
				logNamespaceOperation("Getting plan disk transfers", namespace, allNamespaces)
				logOutputFormat(outputFormatFlag.GetValue())

				return plan.ListDisks(ctx, kubeConfigFlags, planName, namespace, watch)
			}

			// Default behavior: list plans

			// Log the operation being performed
			if planName != "" {
				logNamespaceOperation("Getting plan", namespace, allNamespaces)
			} else {
				logNamespaceOperation("Getting plans", namespace, allNamespaces)
			}
			logOutputFormat(outputFormatFlag.GetValue())

			return plan.List(ctx, kubeConfigFlags, namespace, watch, outputFormatFlag.GetValue(), planName, globalConfig.GetUseUTC())
		},
	}

	cmd.Flags().StringVarP(&planName, "name", "M", "", "Plan name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")
	cmd.Flags().BoolVar(&vms, "vms", false, "Get VMs status in the migration plan (requires plan NAME)")
	cmd.Flags().BoolVar(&disk, "disk", false, "Get disk transfer status in the migration plan (requires plan NAME)")
	cmd.Flags().BoolVar(&vmsTable, "vms-table", false, "Show all VMs across plans in a flat table with source/target inventory details")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (only with --vms-table)")
	help.MarkMCPHidden(cmd, "watch", "vms-table")

	// Add completion for name and output format flags
	if err := cmd.RegisterFlagCompletionFunc("name", completion.PlanNameCompletion(kubeConfigFlags)); err != nil {
		panic(err)
	}
	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
