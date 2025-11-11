package delete

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewDeleteCmd creates the delete command with all its subcommands
func NewDeleteCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete",
		Short:        "Delete resources",
		Long:         `Delete resources like mappings, plans, and providers`,
		SilenceUsage: true,
	}

	// Add mapping subcommand with plural alias
	mappingCmd := NewMappingCmd(kubeConfigFlags)
	mappingCmd.Aliases = []string{"mappings"}
	cmd.AddCommand(mappingCmd)

	// Add plan subcommand with plural alias
	planCmd := NewPlanCmd(kubeConfigFlags)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)

	// Add provider subcommand with plural alias
	providerCmd := NewProviderCmd(kubeConfigFlags)
	providerCmd.Aliases = []string{"providers"}
	cmd.AddCommand(providerCmd)

	// Add host subcommand with plural alias
	hostCmd := NewHostCmd(kubeConfigFlags)
	hostCmd.Aliases = []string{"hosts"}
	cmd.AddCommand(hostCmd)

	// Add hook subcommand with plural alias
	hookCmd := NewHookCmd(kubeConfigFlags)
	hookCmd.Aliases = []string{"hooks"}
	cmd.AddCommand(hookCmd)

	return cmd
}
