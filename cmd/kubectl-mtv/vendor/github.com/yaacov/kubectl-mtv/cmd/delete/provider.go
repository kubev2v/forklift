package delete

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewProviderCmd creates the provider deletion command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var providerNames []string

	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Delete one or more providers",
		Long: `Delete one or more MTV providers.

Deleting a provider removes its connection to the source or target environment.
Ensure no migration plans reference the provider before deletion.`,
		Example: `  # Delete a provider
  kubectl-mtv delete provider --name vsphere-prod

  # Delete multiple providers
  kubectl-mtv delete providers --name provider1,provider2

  # Delete all providers in namespace
  kubectl-mtv delete providers --all`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --all and --name are mutually exclusive
			if all && len(providerNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(providerNames) == 0 {
				return errors.New("either --name or --all is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			if all {
				// Get all provider names from the namespace
				var err error
				providerNames, err = client.GetAllProviderNames(cmd.Context(), kubeConfigFlags, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all provider names: %v", err)
				}
				if len(providerNames) == 0 {
					fmt.Printf("No providers found in namespace %s\n", namespace)
					return nil
				}
			}

			// Loop over each provider name and delete it
			for _, name := range providerNames {
				err := provider.Delete(kubeConfigFlags, name, namespace)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all providers in the namespace")
	cmd.Flags().StringSliceVarP(&providerNames, "name", "M", nil, "Provider name(s) to delete (comma-separated, e.g. \"prov1,prov2\")")
	cmd.Flags().StringSliceVar(&providerNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.ProviderNameCompletion(kubeConfigFlags))

	return cmd
}
