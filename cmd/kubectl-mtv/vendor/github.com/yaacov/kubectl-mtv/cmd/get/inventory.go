package get

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewInventoryCmd creates the inventory command with all its subcommands
func NewInventoryCmd(kubeConfigFlags *genericclioptions.ConfigFlags, getGlobalConfig func() GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "inventory",
		Short:        "Get inventory resources",
		Long:         `Get inventory resources from providers`,
		SilenceUsage: true,
	}

	// Add general inventory resources
	hostCmd := NewInventoryHostCmd(kubeConfigFlags, getGlobalConfig)
	hostCmd.Aliases = []string{"hosts"}
	cmd.AddCommand(hostCmd)

	namespaceCmd := NewInventoryNamespaceCmd(kubeConfigFlags, getGlobalConfig)
	namespaceCmd.Aliases = []string{"namespaces"}
	cmd.AddCommand(namespaceCmd)

	networkCmd := NewInventoryNetworkCmd(kubeConfigFlags, getGlobalConfig)
	networkCmd.Aliases = []string{"networks"}
	cmd.AddCommand(networkCmd)

	storageCmd := NewInventoryStorageCmd(kubeConfigFlags, getGlobalConfig)
	storageCmd.Aliases = []string{"storages"}
	cmd.AddCommand(storageCmd)

	vmCmd := NewInventoryVMCmd(kubeConfigFlags, getGlobalConfig)
	vmCmd.Aliases = []string{"vms"}
	cmd.AddCommand(vmCmd)

	datacenterCmd := NewInventoryDataCenterCmd(kubeConfigFlags, getGlobalConfig)
	datacenterCmd.Aliases = []string{"datacenters"}
	cmd.AddCommand(datacenterCmd)

	clusterCmd := NewInventoryClusterCmd(kubeConfigFlags, getGlobalConfig)
	clusterCmd.Aliases = []string{"clusters"}
	cmd.AddCommand(clusterCmd)

	diskCmd := NewInventoryDiskCmd(kubeConfigFlags, getGlobalConfig)
	diskCmd.Aliases = []string{"disks"}
	cmd.AddCommand(diskCmd)

	// Add profile resources
	diskProfileCmd := NewInventoryDiskProfileCmd(kubeConfigFlags, getGlobalConfig)
	diskProfileCmd.Aliases = []string{"diskprofiles", "disk-profiles"}
	cmd.AddCommand(diskProfileCmd)

	nicProfileCmd := NewInventoryNICProfileCmd(kubeConfigFlags, getGlobalConfig)
	nicProfileCmd.Aliases = []string{"nicprofiles", "nic-profiles"}
	cmd.AddCommand(nicProfileCmd)

	// Add OpenStack-specific resources
	instanceCmd := NewInventoryInstanceCmd(kubeConfigFlags, getGlobalConfig)
	instanceCmd.Aliases = []string{"instances"}
	cmd.AddCommand(instanceCmd)

	imageCmd := NewInventoryImageCmd(kubeConfigFlags, getGlobalConfig)
	imageCmd.Aliases = []string{"images"}
	cmd.AddCommand(imageCmd)

	flavorCmd := NewInventoryFlavorCmd(kubeConfigFlags, getGlobalConfig)
	flavorCmd.Aliases = []string{"flavors"}
	cmd.AddCommand(flavorCmd)

	projectCmd := NewInventoryProjectCmd(kubeConfigFlags, getGlobalConfig)
	projectCmd.Aliases = []string{"projects"}
	cmd.AddCommand(projectCmd)

	volumeCmd := NewInventoryVolumeCmd(kubeConfigFlags, getGlobalConfig)
	volumeCmd.Aliases = []string{"volumes"}
	cmd.AddCommand(volumeCmd)

	volumeTypeCmd := NewInventoryVolumeTypeCmd(kubeConfigFlags, getGlobalConfig)
	volumeTypeCmd.Aliases = []string{"volumetypes", "volume-types"}
	cmd.AddCommand(volumeTypeCmd)

	snapshotCmd := NewInventorySnapshotCmd(kubeConfigFlags, getGlobalConfig)
	snapshotCmd.Aliases = []string{"snapshots"}
	cmd.AddCommand(snapshotCmd)

	subnetCmd := NewInventorySubnetCmd(kubeConfigFlags, getGlobalConfig)
	subnetCmd.Aliases = []string{"subnets"}
	cmd.AddCommand(subnetCmd)

	// Add vSphere-specific resources
	datastoreCmd := NewInventoryDatastoreCmd(kubeConfigFlags, getGlobalConfig)
	datastoreCmd.Aliases = []string{"datastores"}
	cmd.AddCommand(datastoreCmd)

	resourcePoolCmd := NewInventoryResourcePoolCmd(kubeConfigFlags, getGlobalConfig)
	resourcePoolCmd.Aliases = []string{"resourcepools", "resource-pools"}
	cmd.AddCommand(resourcePoolCmd)

	folderCmd := NewInventoryFolderCmd(kubeConfigFlags, getGlobalConfig)
	folderCmd.Aliases = []string{"folders"}
	cmd.AddCommand(folderCmd)

	// Add Kubernetes-specific resources
	pvcCmd := NewInventoryPVCCmd(kubeConfigFlags, getGlobalConfig)
	pvcCmd.Aliases = []string{"pvcs", "persistentvolumeclaims"}
	cmd.AddCommand(pvcCmd)

	dataVolumeCmd := NewInventoryDataVolumeCmd(kubeConfigFlags, getGlobalConfig)
	dataVolumeCmd.Aliases = []string{"datavolumes", "data-volumes"}
	cmd.AddCommand(dataVolumeCmd)

	// Add provider inventory
	providerCmd := NewInventoryProviderCmd(kubeConfigFlags, getGlobalConfig)
	providerCmd.Aliases = []string{"providers"}
	cmd.AddCommand(providerCmd)

	return cmd
}
