package create

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// NewMappingCmd creates the mapping creation command with subcommands
func NewMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mapping",
		Short:        "Create a new mapping",
		Long:         `Create a new network or storage mapping`,
		SilenceUsage: true,
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newNetworkMappingCmd(kubeConfigFlags))
	cmd.AddCommand(newStorageMappingCmd(kubeConfigFlags))

	return cmd
}

// newNetworkMappingCmd creates the network mapping subcommand
func newNetworkMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var sourceProvider, targetProvider string
	var networkPairs string
	var inventoryURL string

	cmd := &cobra.Command{
		Use:          "network NAME",
		Short:        "Create a new network mapping",
		Long:         `Create a new network mapping between source and target providers`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// If inventoryURL is empty, try to discover it
			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(cmd.Context(), kubeConfigFlags, namespace)
			}

			return mapping.CreateNetwork(kubeConfigFlags, name, namespace, sourceProvider, targetProvider, networkPairs, inventoryURL)
		},
	}

	cmd.Flags().StringVarP(&sourceProvider, "source", "S", "", "Source provider name")
	cmd.Flags().StringVarP(&targetProvider, "target", "T", "", "Target provider name")
	cmd.Flags().StringVar(&networkPairs, "network-pairs", "", "Network mapping pairs in format 'source:target-namespace/target-network', 'source:target-network', 'source:default', or 'source:ignored' (comma-separated)")
	cmd.Flags().StringVarP(&inventoryURL, "inventory-url", "i", os.Getenv("MTV_INVENTORY_URL"), "Base URL for the inventory service")

	return cmd
}

// newStorageMappingCmd creates the storage mapping subcommand
func newStorageMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var sourceProvider, targetProvider string
	var storagePairs string
	var inventoryURL string
	var defaultVolumeMode string
	var defaultAccessMode string
	var defaultOffloadPlugin string
	var defaultOffloadSecret string
	var defaultOffloadVendor string

	// Offload secret creation flags
	var offloadVSphereUsername, offloadVSpherePassword, offloadVSphereURL string
	var offloadStorageUsername, offloadStoragePassword, offloadStorageEndpoint string
	var offloadCACert string
	var offloadInsecureSkipTLS bool

	cmd := &cobra.Command{
		Use:          "storage NAME",
		Short:        "Create a new storage mapping",
		Long:         `Create a new storage mapping between source and target providers`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// If inventoryURL is empty, try to discover it
			if inventoryURL == "" {
				inventoryURL = client.DiscoverInventoryURL(cmd.Context(), kubeConfigFlags, namespace)
			}

			return mapping.CreateStorageWithOptions(mapping.StorageCreateOptions{
				ConfigFlags:          kubeConfigFlags,
				Name:                 name,
				Namespace:            namespace,
				SourceProvider:       sourceProvider,
				TargetProvider:       targetProvider,
				StoragePairs:         storagePairs,
				InventoryURL:         inventoryURL,
				DefaultVolumeMode:    defaultVolumeMode,
				DefaultAccessMode:    defaultAccessMode,
				DefaultOffloadPlugin: defaultOffloadPlugin,
				DefaultOffloadSecret: defaultOffloadSecret,
				DefaultOffloadVendor: defaultOffloadVendor,
				// Offload secret creation options
				OffloadVSphereUsername: offloadVSphereUsername,
				OffloadVSpherePassword: offloadVSpherePassword,
				OffloadVSphereURL:      offloadVSphereURL,
				OffloadStorageUsername: offloadStorageUsername,
				OffloadStoragePassword: offloadStoragePassword,
				OffloadStorageEndpoint: offloadStorageEndpoint,
				OffloadCACert:          offloadCACert,
				OffloadInsecureSkipTLS: offloadInsecureSkipTLS,
			})
		},
	}

	cmd.Flags().StringVarP(&sourceProvider, "source", "S", "", "Source provider name")
	cmd.Flags().StringVarP(&targetProvider, "target", "T", "", "Target provider name")
	cmd.Flags().StringVar(&storagePairs, "storage-pairs", "", "Storage mapping pairs in format 'source:storage-class[;volumeMode=Block|Filesystem][;accessMode=ReadWriteOnce|ReadWriteMany|ReadOnlyMany][;offloadPlugin=vsphere][;offloadSecret=secret-name][;offloadVendor=vantara|ontap|...]' (comma-separated pairs, semicolon-separated parameters)")
	cmd.Flags().StringVar(&defaultVolumeMode, "default-volume-mode", "", "Default volume mode for all storage pairs (Filesystem|Block)")
	cmd.Flags().StringVar(&defaultAccessMode, "default-access-mode", "", "Default access mode for all storage pairs (ReadWriteOnce|ReadWriteMany|ReadOnlyMany)")
	cmd.Flags().StringVar(&defaultOffloadPlugin, "default-offload-plugin", "", "Default offload plugin type for all storage pairs (vsphere)")
	cmd.Flags().StringVar(&defaultOffloadSecret, "default-offload-secret", "", "Existing offload secret name to use (creates new secret if not provided and offload credentials given)")
	cmd.Flags().StringVar(&defaultOffloadVendor, "default-offload-vendor", "", "Default offload plugin vendor for all storage pairs (flashsystem|vantara|ontap|primera3par|pureFlashArray|powerflex|powermax|powerstore|infinibox)")
	cmd.Flags().StringVarP(&inventoryURL, "inventory-url", "i", os.Getenv("MTV_INVENTORY_URL"), "Base URL for the inventory service")

	// Offload secret creation flags
	cmd.Flags().StringVar(&offloadVSphereUsername, "offload-vsphere-username", "", "vSphere username for offload secret (creates new secret if no --default-offload-secret provided)")
	cmd.Flags().StringVar(&offloadVSpherePassword, "offload-vsphere-password", "", "vSphere password for offload secret")
	cmd.Flags().StringVar(&offloadVSphereURL, "offload-vsphere-url", "", "vSphere vCenter URL for offload secret")
	cmd.Flags().StringVar(&offloadStorageUsername, "offload-storage-username", "", "Storage array username for offload secret")
	cmd.Flags().StringVar(&offloadStoragePassword, "offload-storage-password", "", "Storage array password for offload secret")
	cmd.Flags().StringVar(&offloadStorageEndpoint, "offload-storage-endpoint", "", "Storage array management endpoint URL for offload secret")
	cmd.Flags().StringVar(&offloadCACert, "offload-cacert", "", "CA certificate for offload secret (use @filename to load from file)")
	cmd.Flags().BoolVar(&offloadInsecureSkipTLS, "offload-insecure-skip-tls", false, "Skip TLS verification for offload connections")

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

	return cmd
}
