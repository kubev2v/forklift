package get

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewInventoryJobTemplateCmd creates the get inventory job-template command
func NewInventoryJobTemplateCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	outputFormatFlag := flags.NewOutputFormatTypeFlag()
	var query string

	cmd := &cobra.Command{
		Use:   "job-template",
		Short: "Get AAP job templates from the inventory service",
		Long: `Get Ansible Automation Platform (AAP) job templates available for migration hooks.

This endpoint requires AAP to be configured on the ForkliftController
(aap_url and aap_token_secret_name settings).

Examples:
  # List all AAP job templates
  kubectl-mtv get inventory job-template

  # Filter job templates by name
  kubectl-mtv get inventory job-template --query "where name ~= 'migration.*'"

  # Output as JSON
  kubectl-mtv get inventory job-template --output json`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 280*time.Second)
			defer cancel()

			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return inventory.ListJobTemplates(ctx, kubeConfigFlags, inventoryURL, outputFormatFlag.GetValue(), query, inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query filter using TSL syntax (e.g. \"where name ~= 'prod-.*'\")")

	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
