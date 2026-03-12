package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/describe/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewProviderCmd creates the provider description command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var name string
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Describe a migration provider",
		Long: `Display detailed information about a migration provider.

Shows provider configuration, type, URL, connection status, conditions,
secret reference, and provider-specific settings (VDDK, SDK endpoint, etc.).`,
		Example: `  # Describe a provider
  kubectl-mtv describe provider --name vsphere-prod

  # Describe a provider in JSON format
  kubectl-mtv describe provider --name vsphere-prod --output json`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			namespace := client.ResolveNamespace(globalConfig.GetKubeConfigFlags())
			return provider.Describe(cmd.Context(), globalConfig.GetKubeConfigFlags(), name, namespace, globalConfig.GetUseUTC(), outputFormatFlag.GetValue())
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Provider name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", flags.OutputFormatHelp)

	_ = cmd.RegisterFlagCompletionFunc("name", completion.ProviderNameCompletion(kubeConfigFlags))
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}
