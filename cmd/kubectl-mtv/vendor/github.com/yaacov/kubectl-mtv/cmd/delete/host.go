package delete

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/host"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewHostCmd creates the delete host command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:               "host [NAME...] [--all]",
		Short:             "Delete one or more migration hosts",
		Args:              flags.ValidateAllFlagArgs(func() bool { return all }, 1),
		SilenceUsage:      true,
		ValidArgsFunction: completion.HostResourceNameCompletion(kubeConfigFlags),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			var hostNames []string
			if all {
				// Get all host names from the namespace
				var err error
				hostNames, err = client.GetAllHostNames(cmd.Context(), kubeConfigFlags, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all host names: %v", err)
				}
				if len(hostNames) == 0 {
					fmt.Printf("No hosts found in namespace %s\n", namespace)
					return nil
				}
			} else {
				hostNames = args
			}

			// Loop over each host name and delete it
			for _, name := range hostNames {
				err := host.Delete(kubeConfigFlags, name, namespace)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all migration hosts in the namespace")

	return cmd
}
