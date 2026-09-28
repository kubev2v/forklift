package testutil

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

const (
	TestSubscription  = "00000000-0000-0000-0000-000000000000"
	TestResourceGroup = "test-rg"
	TestTenant        = "00000000-0000-0000-0000-000000000001"
	TestLocation      = "eastus"
)

func NewTestVM(name, vmSize string) *armcompute.VirtualMachine {
	diskName := name + "-osdisk"
	diskID := "/subscriptions/" + TestSubscription + "/resourceGroups/" + TestResourceGroup + "/providers/Microsoft.Compute/disks/" + diskName

	return &armcompute.VirtualMachine{
		ID:       to.Ptr("/subscriptions/" + TestSubscription + "/resourceGroups/" + TestResourceGroup + "/providers/Microsoft.Compute/virtualMachines/" + name),
		Name:     to.Ptr(name),
		Location: to.Ptr(TestLocation),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(vmSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Offer: to.Ptr("CentOS"),
					SKU:   to.Ptr("7.9"),
				},
				OSDisk: &armcompute.OSDisk{
					Name:       to.Ptr(diskName),
					DiskSizeGB: to.Ptr[int32](128),
					OSType:     to.Ptr(armcompute.OperatingSystemTypesLinux),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						ID:                 to.Ptr(diskID),
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
				DataDisks: []*armcompute.DataDisk{},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr("/subscriptions/" + TestSubscription + "/resourceGroups/" + TestResourceGroup + "/providers/Microsoft.Network/networkInterfaces/" + name + "-nic"),
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							Primary: to.Ptr(true),
						},
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName: to.Ptr(name),
			},
		},
		Tags: map[string]*string{
			"Name": to.Ptr(name),
		},
		Zones: []*string{to.Ptr("1")},
	}
}

func NewTestVMSize(name string, cores, memoryMB int32) *armcompute.VirtualMachineSize {
	return &armcompute.VirtualMachineSize{
		Name:          to.Ptr(name),
		NumberOfCores: to.Ptr(cores),
		MemoryInMB:    to.Ptr(memoryMB),
	}
}

func NewTestDisk(name string, sizeGB int32, sku armcompute.DiskStorageAccountTypes) *armcompute.Disk {
	return &armcompute.Disk{
		ID:       to.Ptr("/subscriptions/" + TestSubscription + "/resourceGroups/" + TestResourceGroup + "/providers/Microsoft.Compute/disks/" + name),
		Name:     to.Ptr(name),
		Location: to.Ptr(TestLocation),
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(sku),
		},
		Properties: &armcompute.DiskProperties{
			DiskSizeGB:        to.Ptr(sizeGB),
			DiskState:         to.Ptr(armcompute.DiskStateAttached),
			OSType:            to.Ptr(armcompute.OperatingSystemTypesLinux),
			ProvisioningState: to.Ptr("Succeeded"),
		},
		Zones: []*string{to.Ptr("1")},
	}
}

func NewTestVNet(name, addressPrefix string) *armnetwork.VirtualNetwork {
	return &armnetwork.VirtualNetwork{
		ID:       to.Ptr("/subscriptions/" + TestSubscription + "/resourceGroups/" + TestResourceGroup + "/providers/Microsoft.Network/virtualNetworks/" + name),
		Name:     to.Ptr(name),
		Location: to.Ptr(TestLocation),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr(addressPrefix)},
			},
		},
	}
}

func NewTestSubnet(name, addressPrefix string) *armnetwork.Subnet {
	return &armnetwork.Subnet{
		ID:   to.Ptr("/subscriptions/" + TestSubscription + "/resourceGroups/" + TestResourceGroup + "/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/" + name),
		Name: to.Ptr(name),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr(addressPrefix),
		},
	}
}
