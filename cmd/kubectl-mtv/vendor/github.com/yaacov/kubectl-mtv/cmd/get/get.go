package get

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// GlobalConfigGetter defines the interface for getting global configuration
type GlobalConfigGetter interface {
	GetVerbosity() int
	GetAllNamespaces() bool
	GetUseUTC() bool
	GetKubeConfigFlags() *genericclioptions.ConfigFlags
}

// logInfof logs formatted informational messages at verbosity level 1
func logInfof(format string, args ...interface{}) {
	klog.V(1).Infof(format, args...)
}

// logDebugf logs formatted debug messages at verbosity level 2
func logDebugf(format string, args ...interface{}) {
	klog.V(2).Infof(format, args...)
}

// logNamespaceOperation logs namespace-specific operations with consistent formatting
func logNamespaceOperation(operation string, namespace string, allNamespaces bool) {
	if allNamespaces {
		logInfof("%s from all namespaces", operation)
	} else {
		logInfof("%s from namespace: %s", operation, namespace)
	}
}

// logOutputFormat logs the output format being used
func logOutputFormat(format string) {
	logDebugf("Output format: %s", format)
}

// NewGetCmd creates the get command with all its subcommands
func NewGetCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Short:        "Get resources",
		Long:         `Get various MTV resources including plans, providers, mappings, and inventory`,
		SilenceUsage: true,
	}

	// Add plan subcommand with plural alias
	planCmd := NewPlanCmd(kubeConfigFlags, getGlobalConfig)
	planCmd.Aliases = []string{"plans"}
	cmd.AddCommand(planCmd)

	// Add provider subcommand with plural alias
	providerCmd := NewProviderCmd(kubeConfigFlags, getGlobalConfig)
	providerCmd.Aliases = []string{"providers"}
	cmd.AddCommand(providerCmd)

	// Add mapping subcommand with plural alias
	mappingCmd := NewMappingCmd(kubeConfigFlags, getGlobalConfig)
	mappingCmd.Aliases = []string{"mappings"}
	cmd.AddCommand(mappingCmd)

	// Add host subcommand with plural alias
	hostCmd := NewHostCmd(kubeConfigFlags, getGlobalConfig)
	hostCmd.Aliases = []string{"hosts"}
	cmd.AddCommand(hostCmd)

	// Add hook subcommand with plural alias
	hookCmd := NewHookCmd(kubeConfigFlags, getGlobalConfig)
	hookCmd.Aliases = []string{"hooks"}
	cmd.AddCommand(hookCmd)

	// Add inventory subcommand
	cmd.AddCommand(NewInventoryCmd(kubeConfigFlags, getGlobalConfig))

	return cmd
}
