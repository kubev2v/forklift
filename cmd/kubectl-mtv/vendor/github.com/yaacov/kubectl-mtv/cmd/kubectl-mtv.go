package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"

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
	"github.com/yaacov/kubectl-mtv/cmd/health"
	"github.com/yaacov/kubectl-mtv/cmd/help"
	"github.com/yaacov/kubectl-mtv/cmd/mcpserver"
	"github.com/yaacov/kubectl-mtv/cmd/patch"
	"github.com/yaacov/kubectl-mtv/cmd/settings"
	"github.com/yaacov/kubectl-mtv/cmd/start"
	"github.com/yaacov/kubectl-mtv/cmd/unarchive"
	"github.com/yaacov/kubectl-mtv/cmd/version"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	pkgversion "github.com/yaacov/kubectl-mtv/pkg/version"
)

// GlobalConfig holds global configuration flags that are passed to all subcommands
type GlobalConfig struct {
	Verbosity                int
	AllNamespaces            bool
	UseUTC                   bool
	InventoryURL             string
	InventoryInsecureSkipTLS bool
	KubeConfigFlags          *genericclioptions.ConfigFlags
	discoveredInventoryURL   string // cached discovered URL
	inventoryURLResolved     bool   // flag to track if we've attempted discovery
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

// GetInventoryURL returns the inventory service URL, auto-discovering if necessary
// This method will automatically discover the URL from OpenShift routes if:
// 1. No URL was provided via flag or environment variable
// 2. Discovery hasn't been attempted yet
func (g *GlobalConfig) GetInventoryURL() string {
	// If explicitly set via flag or env var, return it
	if g.InventoryURL != "" {
		return g.InventoryURL
	}

	// Return cached discovered URL if we already tried discovery
	if g.inventoryURLResolved {
		return g.discoveredInventoryURL
	}

	// Mark as resolved to avoid repeated attempts
	g.inventoryURLResolved = true

	// Attempt auto-discovery from OpenShift routes
	// Note: This uses the default namespace from kubeconfig
	namespace := ""
	if g.KubeConfigFlags.Namespace != nil && *g.KubeConfigFlags.Namespace != "" {
		namespace = *g.KubeConfigFlags.Namespace
	}

	// Use context.Background() for discovery as we don't have a command context here
	discoveredURL := client.DiscoverInventoryURL(context.Background(), g.KubeConfigFlags, namespace)

	if discoveredURL != "" {
		klog.V(2).Infof("Auto-discovered inventory URL: %s", discoveredURL)
		g.discoveredInventoryURL = discoveredURL
	} else {
		klog.V(2).Info("No inventory URL provided and auto-discovery failed (this is expected on non-OpenShift clusters)")
	}

	return g.discoveredInventoryURL
}

// GetInventoryInsecureSkipTLS returns whether to skip TLS verification for inventory service
func (g *GlobalConfig) GetInventoryInsecureSkipTLS() bool {
	return g.InventoryInsecureSkipTLS
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

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Export clientVersion to pkg/version for use by other packages
	pkgversion.ClientVersion = clientVersion

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
	rootCmd.PersistentFlags().StringVarP(&globalConfig.InventoryURL, "inventory-url", "i", os.Getenv("MTV_INVENTORY_URL"), "Base URL for the inventory service")
	rootCmd.PersistentFlags().BoolVar(&globalConfig.InventoryInsecureSkipTLS, "inventory-insecure-skip-tls", os.Getenv("MTV_INVENTORY_INSECURE_SKIP_TLS") == "true", "Skip TLS verification for inventory service connections")

	// Add standard commands for various resources - directly using package functions
	rootCmd.AddCommand(get.NewGetCmd(kubeConfigFlags, globalConfig))
	rootCmd.AddCommand(delete.NewDeleteCmd(kubeConfigFlags))
	rootCmd.AddCommand(create.NewCreateCmd(kubeConfigFlags, globalConfig))
	rootCmd.AddCommand(describe.NewDescribeCmd(kubeConfigFlags, globalConfig))
	rootCmd.AddCommand(patch.NewPatchCmd(kubeConfigFlags, globalConfig))

	// Plan commands - directly using package functions
	rootCmd.AddCommand(start.NewStartCmd(kubeConfigFlags, globalConfig))
	rootCmd.AddCommand(cancel.NewCancelCmd(kubeConfigFlags))
	rootCmd.AddCommand(cutover.NewCutoverCmd(kubeConfigFlags))
	rootCmd.AddCommand(archive.NewArchiveCmd(kubeConfigFlags))
	rootCmd.AddCommand(unarchive.NewUnArchiveCmd(kubeConfigFlags))

	// Version command - directly using package function
	rootCmd.AddCommand(version.NewVersionCmd(clientVersion, kubeConfigFlags, globalConfig))

	// Health command - check MTV system health
	rootCmd.AddCommand(health.NewHealthCmd(kubeConfigFlags, globalConfig))

	// Settings command - view and manage ForkliftController settings
	rootCmd.AddCommand(settings.NewSettingsCmd(kubeConfigFlags, globalConfig))

	// MCP Server command - start the Model Context Protocol server
	rootCmd.AddCommand(mcpserver.NewMCPServerCmd())

	// Help command - replace default Cobra help with our enhanced version
	// that supports machine-readable output for MCP server integration
	rootCmd.SetHelpCommand(help.NewHelpCmd(rootCmd, clientVersion))
}
