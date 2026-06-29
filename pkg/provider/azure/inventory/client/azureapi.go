package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

// AzureAPI defines the Azure operations used by the inventory client.
// This interface allows for mocking Azure SDK calls in unit tests.
type AzureAPI interface {
	ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error)
	GetVMInstanceView(ctx context.Context, resourceGroup string, vmName string) (*armcompute.VirtualMachineInstanceView, error)
	ListVMSizes(ctx context.Context, location string) ([]*armcompute.VirtualMachineSize, error)
	ListDisks(ctx context.Context, resourceGroup string) ([]*armcompute.Disk, error)
	ListVirtualNetworks(ctx context.Context, resourceGroup string) ([]*armnetwork.VirtualNetwork, error)
	ListSubnets(ctx context.Context, resourceGroup string, vnetName string) ([]*armnetwork.Subnet, error)
}
