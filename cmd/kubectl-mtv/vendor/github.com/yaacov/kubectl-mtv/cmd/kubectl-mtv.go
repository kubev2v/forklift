package cmd

import (
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/cmd/archive"
	"github.com/yaacov/kubectl-mtv/cmd/cancel"
	"github.com/yaacov/kubectl-mtv/cmd/create"
	"github.com/yaacov/kubectl-mtv/cmd/cutover"
	"github.com/yaacov/kubectl-mtv/cmd/delete"
	"github.com/yaacov/kubectl-mtv/cmd/describe"
	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/cmd/mcpserver"
	"github.com/yaacov/kubectl-mtv/cmd/patch"
	"github.com/yaacov/kubectl-mtv/cmd/start"
	"github.com/yaacov/kubectl-mtv/cmd/unarchive"
	"github.com/yaacov/kubectl-mtv/cmd/version"
)

// GlobalConfig holds global configuration flags that are passed to all subcommands
type GlobalConfig struct {
	Verbosity       int
	AllNamespaces   bool
	UseUTC          bool
	KubeConfigFlags *genericclioptions.ConfigFlags
}

// GetVerbosity returns the verbosity level
func (g *GlobalConfig) GetVerbosity() int {
	return g.Verbosity
}

// GetAllNamespaces returns whether to list resources across all namespaces
func (g *GlobalConfig) GetAllNamespaces() bool {
	return g.AllNamespaces
}

// GetUseUTC returns whether to format times in UTC
func (g *GlobalConfig) GetUseUTC() bool {
	return g.UseUTC
}

// GetKubeConfigFlags returns the Kubernetes configuration flags
func (g *GlobalConfig) GetKubeConfigFlags() *genericclioptions.ConfigFlags {
	return g.KubeConfigFlags
}

var (
	kubeConfigFlags *genericclioptions.ConfigFlags
	rootCmd         *cobra.Command
	globalConfig    *GlobalConfig
	// Version is set via ldflags during build
	clientVersion = "unknown"
)

// logDebugf logs formatted debug messages at verbosity level 2
func logDebugf(format string, args ...interface{}) {
	klog.V(2).Infof(format, args...)
}

// GetGlobalConfig returns the global configuration instance
func GetGlobalConfig() *GlobalConfig {
	return globalConfig
}

// getGlobalConfigGetter returns the global configuration as an interface to avoid circular imports
func getGlobalConfigGetter() get.GlobalConfigGetter {
	return globalConfig
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	kubeConfigFlags = genericclioptions.NewConfigFlags(true)

	// Initialize global configuration
	globalConfig = &GlobalConfig{
		KubeConfigFlags: kubeConfigFlags,
	}

	rootCmd = &cobra.Command{
		Use:   "kubectl-mtv",
		Short: "Migration Toolkit for Virtualization CLI",
		Long: `Migration Toolkit for Virtualization (MTV) CLI
A kubectl plugin for migrating VMs from oVirt, VMware, OpenStack, and OVA files to KubeVirt.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize klog with the verbosity level
			klog.InitFlags(nil)
			if err := flag.Set("v", fmt.Sprintf("%d", globalConfig.Verbosity)); err != nil {
				klog.Warningf("Failed to set klog verbosity: %v", err)
			}

			// Log global configuration if verbosity is enabled
			logDebugf("Global configuration - Verbosity: %d, All Namespaces: %t",
				globalConfig.Verbosity, globalConfig.AllNamespaces)
		},
	}

	kubeConfigFlags.AddFlags(rootCmd.PersistentFlags())

	// Add global flags
	rootCmd.PersistentFlags().IntVarP(&globalConfig.Verbosity, "verbose", "v", 0, "verbose output level (0=silent, 1=info, 2=debug, 3=trace)")
	rootCmd.PersistentFlags().BoolVarP(&globalConfig.AllNamespaces, "all-namespaces", "A", false, "list resources across all namespaces")
	rootCmd.PersistentFlags().BoolVar(&globalConfig.UseUTC, "use-utc", false, "format timestamps in UTC instead of local timezone")

	// Add standard commands for various resources - directly using package functions
	rootCmd.AddCommand(get.NewGetCmd(kubeConfigFlags, getGlobalConfigGetter))
	rootCmd.AddCommand(delete.NewDeleteCmd(kubeConfigFlags))
	rootCmd.AddCommand(create.NewCreateCmd(kubeConfigFlags, globalConfig))
	rootCmd.AddCommand(describe.NewDescribeCmd(kubeConfigFlags, getGlobalConfigGetter))
	rootCmd.AddCommand(patch.NewPatchCmd(kubeConfigFlags))

	// Plan commands - directly using package functions
	rootCmd.AddCommand(start.NewStartCmd(kubeConfigFlags, getGlobalConfigGetter))
	rootCmd.AddCommand(cancel.NewCancelCmd(kubeConfigFlags))
	rootCmd.AddCommand(cutover.NewCutoverCmd(kubeConfigFlags))
	rootCmd.AddCommand(archive.NewArchiveCmd(kubeConfigFlags))
	rootCmd.AddCommand(unarchive.NewUnArchiveCmd(kubeConfigFlags))

	// Version command - directly using package function
	rootCmd.AddCommand(version.NewVersionCmd(clientVersion, kubeConfigFlags))

	// MCP Server command - start the Model Context Protocol server
	rootCmd.AddCommand(mcpserver.NewMCPServerCmd())
}
