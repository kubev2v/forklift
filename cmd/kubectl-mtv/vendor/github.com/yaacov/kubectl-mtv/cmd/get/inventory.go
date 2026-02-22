package get

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewInventoryCmd creates the inventory command with all its subcommands
func NewInventoryCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Get inventory resources",
		Long: `Get inventory resources from providers via the MTV inventory service.

The inventory service provides access to source provider resources (VMs, networks,
storage, hosts, etc.) without directly connecting to the provider. Resources are
cached and can be queried using TSL (Tree Search Language) filters.

Available resource types vary by provider:
  - All providers: vm, network, storage
  - vSphere/oVirt: host, datacenter, cluster, disk
  - vSphere: datastore, folder, resourcepool
  - oVirt: diskprofile, nicprofile
  - OpenStack: instance, image, flavor, project, volume, volumetype, snapshot, subnet
  - OpenShift: namespace, pvc, datavolume
  - EC2: ec2instance, ec2volume, ec2volumetype, ec2network`,
		SilenceUsage: true,
	}

	// Add general inventory resources
	hostCmd := NewInventoryHostCmd(kubeConfigFlags, globalConfig)
	hostCmd.Aliases = []string{"hosts"}
	cmd.AddCommand(hostCmd)

	namespaceCmd := NewInventoryNamespaceCmd(kubeConfigFlags, globalConfig)
	namespaceCmd.Aliases = []string{"namespaces"}
	cmd.AddCommand(namespaceCmd)

	networkCmd := NewInventoryNetworkCmd(kubeConfigFlags, globalConfig)
	networkCmd.Aliases = []string{"networks"}
	cmd.AddCommand(networkCmd)

	storageCmd := NewInventoryStorageCmd(kubeConfigFlags, globalConfig)
	storageCmd.Aliases = []string{"storages"}
	cmd.AddCommand(storageCmd)

	vmCmd := NewInventoryVMCmd(kubeConfigFlags, globalConfig)
	vmCmd.Aliases = []string{"vms"}
	cmd.AddCommand(vmCmd)

	datacenterCmd := NewInventoryDataCenterCmd(kubeConfigFlags, globalConfig)
	datacenterCmd.Aliases = []string{"datacenters"}
	cmd.AddCommand(datacenterCmd)

	clusterCmd := NewInventoryClusterCmd(kubeConfigFlags, globalConfig)
	clusterCmd.Aliases = []string{"clusters"}
	cmd.AddCommand(clusterCmd)

	diskCmd := NewInventoryDiskCmd(kubeConfigFlags, globalConfig)
	diskCmd.Aliases = []string{"disks"}
	cmd.AddCommand(diskCmd)

	// Add profile resources
	diskProfileCmd := NewInventoryDiskProfileCmd(kubeConfigFlags, globalConfig)
	diskProfileCmd.Aliases = []string{"diskprofiles", "disk-profiles"}
	cmd.AddCommand(diskProfileCmd)

	nicProfileCmd := NewInventoryNICProfileCmd(kubeConfigFlags, globalConfig)
	nicProfileCmd.Aliases = []string{"nicprofiles", "nic-profiles"}
	cmd.AddCommand(nicProfileCmd)

	// Add OpenStack-specific resources
	instanceCmd := NewInventoryInstanceCmd(kubeConfigFlags, globalConfig)
	instanceCmd.Aliases = []string{"instances"}
	cmd.AddCommand(instanceCmd)

	imageCmd := NewInventoryImageCmd(kubeConfigFlags, globalConfig)
	imageCmd.Aliases = []string{"images"}
	cmd.AddCommand(imageCmd)

	flavorCmd := NewInventoryFlavorCmd(kubeConfigFlags, globalConfig)
	flavorCmd.Aliases = []string{"flavors"}
	cmd.AddCommand(flavorCmd)

	projectCmd := NewInventoryProjectCmd(kubeConfigFlags, globalConfig)
	projectCmd.Aliases = []string{"projects"}
	cmd.AddCommand(projectCmd)

	volumeCmd := NewInventoryVolumeCmd(kubeConfigFlags, globalConfig)
	volumeCmd.Aliases = []string{"volumes"}
	cmd.AddCommand(volumeCmd)

	volumeTypeCmd := NewInventoryVolumeTypeCmd(kubeConfigFlags, globalConfig)
	volumeTypeCmd.Aliases = []string{"volumetypes", "volume-types"}
	cmd.AddCommand(volumeTypeCmd)

	snapshotCmd := NewInventorySnapshotCmd(kubeConfigFlags, globalConfig)
	snapshotCmd.Aliases = []string{"snapshots"}
	cmd.AddCommand(snapshotCmd)

	subnetCmd := NewInventorySubnetCmd(kubeConfigFlags, globalConfig)
	subnetCmd.Aliases = []string{"subnets"}
	cmd.AddCommand(subnetCmd)

	// Add vSphere-specific resources
	datastoreCmd := NewInventoryDatastoreCmd(kubeConfigFlags, globalConfig)
	datastoreCmd.Aliases = []string{"datastores"}
	cmd.AddCommand(datastoreCmd)

	resourcePoolCmd := NewInventoryResourcePoolCmd(kubeConfigFlags, globalConfig)
	resourcePoolCmd.Aliases = []string{"resourcepools", "resource-pools"}
	cmd.AddCommand(resourcePoolCmd)

	folderCmd := NewInventoryFolderCmd(kubeConfigFlags, globalConfig)
	folderCmd.Aliases = []string{"folders"}
	cmd.AddCommand(folderCmd)

	// Add Kubernetes-specific resources
	pvcCmd := NewInventoryPVCCmd(kubeConfigFlags, globalConfig)
	pvcCmd.Aliases = []string{"pvcs", "persistentvolumeclaims"}
	cmd.AddCommand(pvcCmd)

	dataVolumeCmd := NewInventoryDataVolumeCmd(kubeConfigFlags, globalConfig)
	dataVolumeCmd.Aliases = []string{"datavolumes", "data-volumes"}
	cmd.AddCommand(dataVolumeCmd)

	// Add provider inventory
	providerCmd := NewInventoryProviderCmd(kubeConfigFlags, globalConfig)
	providerCmd.Aliases = []string{"providers"}
	cmd.AddCommand(providerCmd)

	// Add EC2-specific resources
	ec2InstanceCmd := NewInventoryEC2InstanceCmd(kubeConfigFlags, globalConfig)
	ec2InstanceCmd.Aliases = []string{"ec2-instances"}
	cmd.AddCommand(ec2InstanceCmd)

	ec2VolumeCmd := NewInventoryEC2VolumeCmd(kubeConfigFlags, globalConfig)
	ec2VolumeCmd.Aliases = []string{"ec2-volumes"}
	cmd.AddCommand(ec2VolumeCmd)

	ec2VolumeTypeCmd := NewInventoryEC2VolumeTypeCmd(kubeConfigFlags, globalConfig)
	ec2VolumeTypeCmd.Aliases = []string{"ec2-volumetypes", "ec2-volume-types"}
	cmd.AddCommand(ec2VolumeTypeCmd)

	ec2NetworkCmd := NewInventoryEC2NetworkCmd(kubeConfigFlags, globalConfig)
	ec2NetworkCmd.Aliases = []string{"ec2-networks"}
	cmd.AddCommand(ec2NetworkCmd)

	return cmd
}
