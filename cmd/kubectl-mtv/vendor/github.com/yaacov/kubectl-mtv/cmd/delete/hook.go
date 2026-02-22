package delete

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/delete/hook"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewHookCmd creates the delete hook command
func NewHookCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var all bool
	var hookNames []string

	cmd := &cobra.Command{
		Use:          "hook",
		Short:        "Delete one or more migration hooks",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --all and --name are mutually exclusive
			if all && len(hookNames) > 0 {
				return errors.New("cannot use --name with --all")
			}
			if !all && len(hookNames) == 0 {
				return errors.New("either --name or --all is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			if all {
				// Get all hook names from the namespace
				var err error
				hookNames, err = client.GetAllHookNames(cmd.Context(), kubeConfigFlags, namespace)
				if err != nil {
					return fmt.Errorf("failed to get all hook names: %v", err)
				}
				if len(hookNames) == 0 {
					fmt.Printf("No hooks found in namespace %s\n", namespace)
					return nil
				}
			}

			// Loop over each hook name and delete it
			for _, name := range hookNames {
				err := hook.Delete(kubeConfigFlags, name, namespace)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all migration hooks in the namespace")
	cmd.Flags().StringSliceVarP(&hookNames, "name", "M", nil, "Hook name(s) to delete (comma-separated, e.g. \"hook1,hook2\")")
	cmd.Flags().StringSliceVar(&hookNames, "names", nil, "Alias for --name")
	_ = cmd.Flags().MarkHidden("names")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.HookResourceNameCompletion(kubeConfigFlags))

	return cmd
}
