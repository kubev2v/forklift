package create

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// NewMappingCmd creates the mapping creation command with subcommands
func NewMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mapping",
		Short: "Create a new mapping",
		Long: `Create a new network or storage mapping.

Mappings define how source provider resources (networks, storage) are translated
to target OpenShift resources. Use 'mapping network' or 'mapping storage' to
create specific mapping types.`,
		SilenceUsage: true,
	}

	// Add subcommands for network and storage
	cmd.AddCommand(newNetworkMappingCmd(kubeConfigFlags, globalConfig))
	cmd.AddCommand(newStorageMappingCmd(kubeConfigFlags, globalConfig))

	return cmd
}

// newNetworkMappingCmd creates the network mapping subcommand
func newNetworkMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var sourceProvider, targetProvider string
	var networkPairs string

	cmd := &cobra.Command{
		Use:   "network NAME",
		Short: "Create a new network mapping",
		Long: `Create a new network mapping between source and target providers.

Network mappings translate source VM network connections to target network
attachment definitions (NADs) or pod networking ('default').

Pair formats:
  - source:target-namespace/target-network - Map to specific NAD
  - source:target-network - Map to NAD in same namespace
  - source:default - Map to pod networking
  - source:ignored - Skip this network`,
		Example: `  # Create a network mapping to pod networking
  kubectl-mtv create mapping network my-net-map \
    --source vsphere-prod \
    --target host \
    --network-pairs "VM Network:default"

  # Create a network mapping to a specific NAD
  kubectl-mtv create mapping network my-net-map \
    --source vsphere-prod \
    --target host \
    --network-pairs "VM Network:openshift-cnv/br-external,Management:default"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return mapping.CreateNetworkWithInsecure(kubeConfigFlags, name, namespace, sourceProvider, targetProvider, networkPairs, inventoryURL, inventoryInsecureSkipTLS)
		},
	}

	cmd.Flags().StringVarP(&sourceProvider, "source", "S", "", "Source provider name")
	cmd.Flags().StringVarP(&targetProvider, "target", "T", "", "Target provider name")
	cmd.Flags().StringVar(&networkPairs, "network-pairs", "", "Network mapping pairs in format 'source:target-namespace/target-network', 'source:target-network', 'source:default', or 'source:ignored' (comma-separated)")

	return cmd
}

// newStorageMappingCmd creates the storage mapping subcommand
func newStorageMappingCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	var sourceProvider, targetProvider string
	var storagePairs string
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
		Use:   "storage NAME",
		Short: "Create a new storage mapping",
		Long: `Create a new storage mapping between source and target providers.

Storage mappings translate source datastores/storage domains to target Kubernetes
storage classes. Advanced options include volume mode, access mode, and offload
plugin configuration for optimized data transfer.`,
		Example: `  # Create a simple storage mapping
  kubectl-mtv create mapping storage my-storage-map \
    --source vsphere-prod \
    --target host \
    --storage-pairs "datastore1:standard,datastore2:fast"

  # Create a storage mapping with volume mode
  kubectl-mtv create mapping storage my-storage-map \
    --source vsphere-prod \
    --target host \
    --storage-pairs "datastore1:ocs-storagecluster-ceph-rbd" \
    --default-volume-mode Block

  # Create a storage mapping with offload plugin
  kubectl-mtv create mapping storage my-storage-map \
    --source vsphere-prod \
    --target host \
    --storage-pairs "datastore1:ocs-storagecluster-ceph-rbd;offloadPlugin=vsphere;offloadVendor=ontap"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get name from positional argument
			name := args[0]

			// Resolve the appropriate namespace based on context and flags
			namespace := client.ResolveNamespace(kubeConfigFlags)

			// Get inventory URL and insecure skip TLS from global config (auto-discovers if needed)
			inventoryURL := globalConfig.GetInventoryURL()
			inventoryInsecureSkipTLS := globalConfig.GetInventoryInsecureSkipTLS()

			return mapping.CreateStorageWithOptions(mapping.StorageCreateOptions{
				ConfigFlags:              kubeConfigFlags,
				Name:                     name,
				Namespace:                namespace,
				SourceProvider:           sourceProvider,
				TargetProvider:           targetProvider,
				StoragePairs:             storagePairs,
				InventoryURL:             inventoryURL,
				InventoryInsecureSkipTLS: inventoryInsecureSkipTLS,
				DefaultVolumeMode:        defaultVolumeMode,
				DefaultAccessMode:        defaultAccessMode,
				DefaultOffloadPlugin:     defaultOffloadPlugin,
				DefaultOffloadSecret:     defaultOffloadSecret,
				DefaultOffloadVendor:     defaultOffloadVendor,
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
