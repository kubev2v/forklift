package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

type ComputeAPI interface {
	BeginDeallocate(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeallocateResponse], error)
	Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (armcompute.VirtualMachinesClientGetResponse, error)
	InstanceView(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientInstanceViewOptions) (armcompute.VirtualMachinesClientInstanceViewResponse, error)
}

type SnapshotAPI interface {
	BeginCreateOrUpdate(ctx context.Context, resourceGroupName string, snapshotName string, snapshot armcompute.Snapshot, options *armcompute.SnapshotsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armcompute.SnapshotsClientCreateOrUpdateResponse], error)
	Get(ctx context.Context, resourceGroupName string, snapshotName string, options *armcompute.SnapshotsClientGetOptions) (armcompute.SnapshotsClientGetResponse, error)
	BeginDelete(ctx context.Context, resourceGroupName string, snapshotName string, options *armcompute.SnapshotsClientBeginDeleteOptions) (*runtime.Poller[armcompute.SnapshotsClientDeleteResponse], error)
}

var _ ComputeAPI = (*armcompute.VirtualMachinesClient)(nil)
var _ SnapshotAPI = (*armcompute.SnapshotsClient)(nil)
