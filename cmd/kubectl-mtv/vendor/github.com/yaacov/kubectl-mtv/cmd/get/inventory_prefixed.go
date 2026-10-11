package get

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
)

// NewInventoryVSphereCustomFieldDefCmd creates the get inventory vsphere-custom-field-def command.
func NewInventoryVSphereCustomFieldDefCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "vsphere-custom-field-def",
		short:      "Get vSphere custom field definitions from a provider",
		long:       `Get custom field definitions from a vSphere provider's inventory.`,
		logMessage: "Getting vSphere custom field definitions from provider",
		listFunc:   inventory.ListVSphereCustomFieldDefsWithInsecure,
	})
}

// NewInventoryOVirtServerCpuCmd creates the get inventory ovirt-server-cpu command.
func NewInventoryOVirtServerCpuCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "ovirt-server-cpu",
		short:      "Get oVirt server CPU types from a provider",
		long:       `Get server CPU types from an oVirt provider's inventory.`,
		logMessage: "Getting oVirt server CPUs from provider",
		listFunc:   inventory.ListOVirtServerCpusWithInsecure,
	})
}

// NewInventoryOpenStackRegionCmd creates the get inventory openstack-region command.
func NewInventoryOpenStackRegionCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "openstack-region",
		short:      "Get OpenStack regions from a provider",
		long:       `Get regions from an OpenStack provider's inventory.`,
		logMessage: "Getting OpenStack regions from provider",
		listFunc:   inventory.ListOpenStackRegionsWithInsecure,
	})
}

// NewInventoryOpenShiftInstanceTypeCmd creates the get inventory openshift-instance-type command.
func NewInventoryOpenShiftInstanceTypeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "openshift-instance-type",
		short:      "Get OpenShift VM instance types from a provider",
		long:       `Get namespaced VirtualMachineInstancetype resources from an OpenShift provider's inventory.`,
		logMessage: "Getting OpenShift instance types from provider",
		listFunc:   inventory.ListOpenShiftInstanceTypesWithInsecure,
	})
}

// NewInventoryOpenShiftClusterInstanceTypeCmd creates the get inventory openshift-cluster-instance-type command.
func NewInventoryOpenShiftClusterInstanceTypeCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "openshift-cluster-instance-type",
		short:      "Get OpenShift cluster VM instance types from a provider",
		long:       `Get cluster-scoped VirtualMachineClusterInstancetype resources from an OpenShift provider's inventory.`,
		logMessage: "Getting OpenShift cluster instance types from provider",
		listFunc:   inventory.ListOpenShiftClusterInstanceTypesWithInsecure,
	})
}

// NewInventoryOpenShiftKubeVirtCmd creates the get inventory openshift-kubevirt command.
func NewInventoryOpenShiftKubeVirtCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "openshift-kubevirt",
		short:      "Get OpenShift KubeVirt CRs from a provider",
		long:       `Get KubeVirt custom resources from an OpenShift provider's inventory.`,
		logMessage: "Getting OpenShift KubeVirt CRs from provider",
		listFunc:   inventory.ListOpenShiftKubeVirtsWithInsecure,
	})
}

// NewInventoryNutanixImageCmd creates the get inventory nutanix-image command.
func NewInventoryNutanixImageCmd(kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig GlobalConfigGetter) *cobra.Command {
	return newTypedInventoryCmd(kubeConfigFlags, globalConfig, typedInventoryCmdConfig{
		use:        "nutanix-image",
		short:      "Get Nutanix images from a provider",
		long:       `Get images from a Nutanix provider's inventory.`,
		logMessage: "Getting Nutanix images from provider",
		listFunc:   inventory.ListNutanixImagesWithInsecure,
	})
}
