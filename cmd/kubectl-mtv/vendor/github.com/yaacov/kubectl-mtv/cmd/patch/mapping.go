package patch

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/patch/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/completion"
)

// NewMappingCmd creates the mapping patch command with subcommands
func NewMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mapping",
		Short:        "Patch mappings",
		Long:         `Patch network and storage mappings by adding, updating, or removing pairs`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is specified, show help
			return cmd.Help()
		},
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newPatchNetworkMappingCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(newPatchStorageMappingCmd(kubeConfigFlags, globalConfig))

	return cmd
}

// newPatchNetworkMappingCmd creates the patch network mapping subcommand
func newPatchNetworkMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var name string
	var addPairs, updatePairs, removePairs string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Patch a network mapping",
		Long:  `Patch a network mapping by adding, updating, or removing network pairs`,
		Example: `  # Add network pairs to a mapping
  kubectl-mtv patch mapping network --name my-net-map --add-pairs "VM Network:default"

  # Update network pairs
  kubectl-mtv patch mapping network --name my-net-map --update-pairs "VM Network:migration-net"`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required --name flag
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Get inventory URL from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()

			return mapping.PatchNetwork(kubeConfigFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Network mapping name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&addPairs, "add-pairs", "", "Network pairs to add in format 'source:target-namespace/target-network', 'source:target-network', 'source:default', or 'source:ignored' (comma-separated)")
	cmd.Flags().StringVar(&updatePairs, "update-pairs", "", "Network pairs to update in format 'source:target-namespace/target-network', 'source:target-network', 'source:default', or 'source:ignored' (comma-separated)")
	cmd.Flags().StringVar(&removePairs, "remove-pairs", "", "Source network names to remove from mapping (comma-separated)")

	_ = cmd.RegisterFlagCompletionFunc("name", completion.MappingNameCompletion(kubeConfigFlags, "network"))

	return cmd
}

// newPatchStorageMappingCmd creates the patch storage mapping subcommand
func newPatchStorageMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var name string
	var addPairs, updatePairs, removePairs string
	var defaultVolumeMode string
	var defaultAccessMode string
	var defaultOffloadPlugin string
	var defaultOffloadSecret string
	var defaultOffloadVendor string

	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Patch a storage mapping",
		Long:  `Patch a storage mapping by adding, updating, or removing storage pairs`,
		Example: `  # Add storage pairs to a mapping
  kubectl-mtv patch mapping storage --name my-storage-map --add-pairs "datastore1:standard"

  # Update storage pairs
  kubectl-mtv patch mapping storage --name my-storage-map --update-pairs "datastore1:premium"`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required --name flag
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return mapping.PatchStorageWithOptions(kubeConfigFlags, name, namespace, addPairs, updatePairs,
				removePairs, inventoryURL, inventoryInsecureSkipTLS, defaultVolumeMode, defaultAccessMode,
				defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "M", "", "Storage mapping name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&addPairs, "add-pairs", "", "Storage pairs to add in format 'source:storage-class[;volumeMode=Block|Filesystem][;accessMode=ReadWriteOnce|ReadWriteMany|ReadOnlyMany][;offloadPlugin=vsphere][;offloadSecret=secret-name][;offloadVendor=vantara|ontap|...]' (comma-separated pairs, semicolon-separated parameters)")
	cmd.Flags().StringVar(&updatePairs, "update-pairs", "", "Storage pairs to update in format 'source:storage-class[;volumeMode=Block|Filesystem][;accessMode=ReadWriteOnce|ReadWriteMany|ReadOnlyMany][;offloadPlugin=vsphere][;offloadSecret=secret-name][;offloadVendor=vantara|ontap|...]' (comma-separated pairs, semicolon-separated parameters)")
	cmd.Flags().StringVar(&removePairs, "remove-pairs", "", "Source storage names to remove from mapping (comma-separated)")
	cmd.Flags().StringVar(&defaultVolumeMode, "default-volume-mode", "", "Default volume mode for new/updated storage pairs (Filesystem|Block)")
	cmd.Flags().StringVar(&defaultAccessMode, "default-access-mode", "", "Default access mode for new/updated storage pairs (ReadWriteOnce|ReadWriteMany|ReadOnlyMany)")
	cmd.Flags().StringVar(&defaultOffloadPlugin, "default-offload-plugin", "", "Default offload plugin type for new/updated storage pairs (vsphere)")
	cmd.Flags().StringVar(&defaultOffloadSecret, "default-offload-secret", "", "Default offload plugin secret name for new/updated storage pairs")
	cmd.Flags().StringVar(&defaultOffloadVendor, "default-offload-vendor", "", "Default offload plugin vendor for new/updated storage pairs (flashsystem|vantara|ontap|primera3par|pureFlashArray|powerflex|powermax|powerstore|infinibox)")

	// Add completion for volume mode flag
	if err := cmd.RegisterFlagCompletionFunc("default-volume-mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"Filesystem", "Block"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add completion for access mode flag
	if err := cmd.RegisterFlagCompletionFunc("default-access-mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"ReadWriteOnce", "ReadWriteMany", "ReadOnlyMany"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add completion for offload plugin flag
	if err := cmd.RegisterFlagCompletionFunc("default-offload-plugin", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"vsphere"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	// Add completion for offload vendor flag
	if err := cmd.RegisterFlagCompletionFunc("default-offload-vendor", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"flashsystem", "vantara", "ontap", "primera3par", "pureFlashArray", "powerflex", "powermax", "powerstore", "infinibox"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	_ = cmd.RegisterFlagCompletionFunc("name", completion.MappingNameCompletion(kubeConfigFlags, "storage"))

	return cmd
}
