package delete

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/host"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewHostCmd creates the delete host command
func NewHostCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var hostNames []string

	cmd := &cobra.Command{
		Use:          "host",
		Short:        "Delete one or more migration hosts",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --all and --name are mutually exclusive
			if all && len(hostNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(hostNames) == 0 {
				return errors.New("either --name or --all is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

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
	cmd.Flags().StringSliceVarP(&hostNames, "name", "M", nil, "Host name(s) to delete (comma-separated, e.g. \"host1,host2\")")
	cmd.Flags().StringSliceVar(&hostNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HostResourceNameCompletion(kubeConfigFlags))

	return cmd
}
