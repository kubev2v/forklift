package create

import (
	"github.com/spf13/cobra"
	"github.com/yaacov/kubectl-mtv/pkg/util/config"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GlobalConfigGetter is a type alias for the shared config interface.
// This maintains backward compatibility while using the centralized interface definition.
type GlobalConfigGetter = config.InventoryConfigWithVerbosity

// NewCreateCmd creates the create command with all its subcommands
func NewCreateCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create resources",
		Long:         `Create various MTV resources like providers, plans, mappings, and VDDK images`,
		SilenceUsage: true,
	}

	providerCmd := NewProviderCmd(kubeConfigFlags)
	providerCmd.Aliases = []string{"providers"}
	cmd.AddCommand(providerCmd)

	planCmd := NewPlanCmd(kubeConfigFlags, globalConfig)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)

	mappingCmd := NewMappingCmd(kubeConfigFlags, globalConfig)
	mappingCmd.Aliases = []string{"mappings"}
	cmd.AddCommand(mappingCmd)

	hostCmd := NewHostCmd(kubeConfigFlags, globalConfig)
	hostCmd.Aliases = []string{"hosts"}
	cmd.AddCommand(hostCmd)

	hookCmd := NewHookCmd(kubeConfigFlags)
	hookCmd.Aliases = []string{"hooks"}
	cmd.AddCommand(hookCmd)

	cmd.AddCommand(NewVddkCmd(globalConfig, kubeConfigFlags))

	return cmd
}
