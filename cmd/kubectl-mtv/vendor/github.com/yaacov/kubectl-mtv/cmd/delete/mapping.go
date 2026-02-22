package delete

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewMappingCmd creates the mapping deletion command with subcommands
func NewMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mapping",
		Short: "Delete mappings",
		Long: `Delete network and storage mappings.

Mappings define how source resources translate to target resources. Use
'mapping network' or 'mapping storage' to delete specific mapping types.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newDeleteNetworkMappingCmd(kubeConfigFlags))
	cmd.AddCommand(newDeleteStorageMappingCmd(kubeConfigFlags))

	return cmd
}

// newDeleteNetworkMappingCmd creates the delete network mapping subcommand
func newDeleteNetworkMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var mappingNames []string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Delete one or more network mappings",
		Long: `Delete one or more network mappings.

Ensure no migration plans reference the mapping before deletion.`,
		Example: `  # Delete a network mapping
  kubectl-mtv delete mapping network --name my-net-map

  # Delete multiple network mappings
  kubectl-mtv delete mappings network --name map1,map2,map3

  # Delete all network mappings
  kubectl-mtv delete mappings network --all`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --all and --name are mutually exclusive
			if all && len(mappingNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(mappingNames) == 0 {
				return errors.New("either --name or --all is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			if all {
				// Get all network mapping names from the namespace
				var err error
				mappingNames, err = client.GetAllNetworkMappingNames(cmd.Context(), kubeConfigFlags, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all network mapping names: %v", err)
				}
				if len(mappingNames) == 0 {
					fmt.Printf("No network mappings found in namespace %s\n", namespace)
					return nil
				}
			}

			// Loop over each mapping name and delete it
			for _, name := range mappingNames {
				err := mapping.Delete(kubeConfigFlags, name, namespace, "network")
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all network mappings in the namespace")
	cmd.Flags().StringSliceVarP(&mappingNames, "name", "M", nil, "Network mapping name(s) to delete (comma-separated, e.g. \"map1,map2\")")
	cmd.Flags().StringSliceVar(&mappingNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")

	_ = cmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.MappingNameCompletion(kubeConfigFlags, "network")(cmd, args, toComplete)
	})

	return cmd
}

// newDeleteStorageMappingCmd creates the delete storage mapping subcommand
func newDeleteStorageMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var mappingNames []string

	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Delete one or more storage mappings",
		Long: `Delete one or more storage mappings.

Ensure no migration plans reference the mapping before deletion.`,
		Example: `  # Delete a storage mapping
  kubectl-mtv delete mapping storage --name my-storage-map

  # Delete multiple storage mappings
  kubectl-mtv delete mappings storage --name map1,map2,map3

  # Delete all storage mappings
  kubectl-mtv delete mappings storage --all`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --all and --name are mutually exclusive
			if all && len(mappingNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(mappingNames) == 0 {
				return errors.New("either --name or --all is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			if all {
				// Get all storage mapping names from the namespace
				var err error
				mappingNames, err = client.GetAllStorageMappingNames(cmd.Context(), kubeConfigFlags, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all storage mapping names: %v", err)
				}
				if len(mappingNames) == 0 {
					fmt.Printf("No storage mappings found in namespace %s\n", namespace)
					return nil
				}
			}

			// Loop over each mapping name and delete it
			for _, name := range mappingNames {
				err := mapping.Delete(kubeConfigFlags, name, namespace, "storage")
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all storage mappings in the namespace")
	cmd.Flags().StringSliceVarP(&mappingNames, "name", "M", nil, "Storage mapping name(s) to delete (comma-separated, e.g. \"map1,map2\")")
	cmd.Flags().StringSliceVar(&mappingNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")

	_ = cmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.MappingNameCompletion(kubeConfigFlags, "storage")(cmd, args, toComplete)
	})

	return cmd
}
