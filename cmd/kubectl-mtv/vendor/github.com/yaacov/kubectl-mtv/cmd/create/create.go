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

	cmd.AddCommand(NewProviderCmd(kubeConfigFlags))
	cmd.AddCommand(NewPlanCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(NewMappingCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(NewHostCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(NewHookCmd(kubeConfigFlags))
	cmd.AddCommand(NewVddkCmd(globalConfig, kubeConfigFlags))

	return cmd
}
