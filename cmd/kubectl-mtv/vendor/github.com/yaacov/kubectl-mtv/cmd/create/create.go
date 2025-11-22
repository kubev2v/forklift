package create

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GlobalConfigGetter defines the interface for getting global configuration
type GlobalConfigGetter interface {
	GetVerbosity() int
	GetInventoryURL() string
	GetInventoryInsecureSkipTLS() bool
}

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
	cmd.AddCommand(NewVddkCmd(globalConfig))

	return cmd
}
