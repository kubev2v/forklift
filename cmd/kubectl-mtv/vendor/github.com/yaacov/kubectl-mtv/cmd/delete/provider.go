package delete

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewProviderCmd creates the provider deletion command
func NewProviderCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:               "provider [NAME...] [--all]",
		Short:             "Delete one or more providers",
		Args:              flags.ValidateAllFlagArgs(func() bool { return all }, 1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.ProviderNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			var providerNames []string
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
			} else {
				providerNames = args
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

	return cmd
}
