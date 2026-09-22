package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

type FakeAzureAPI struct {
	mu             sync.Mutex
	VMs            []*armcompute.VirtualMachine
	VMSizes        []*armcompute.VirtualMachineSize
	Disks          []*armcompute.Disk
	VNets          []*armnetwork.VirtualNetwork
	Subnets        map[string][]*armnetwork.Subnet // vnetName -> subnets
	Snapshots      map[string]*armcompute.Snapshot // snapName -> snapshot
	DeallocatedVMs map[string]bool
	ListVMsErr     error
	ListDisksErr   error
}

func NewFakeAzureAPI() *FakeAzureAPI {
	return &FakeAzureAPI{
		Subnets:        make(map[string][]*armnetwork.Subnet),
		Snapshots:      make(map[string]*armcompute.Snapshot),
		DeallocatedVMs: make(map[string]bool),
	}
}

func (f *FakeAzureAPI) ListVirtualMachines(_ context.Context, _ string) ([]*armcompute.VirtualMachine, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ListVMsErr != nil {
		return nil, f.ListVMsErr
	}
	return f.VMs, nil
}

func (f *FakeAzureAPI) ListDisks(_ context.Context, _ string) ([]*armcompute.Disk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ListDisksErr != nil {
		return nil, f.ListDisksErr
	}
	return f.Disks, nil
}

func (f *FakeAzureAPI) ListVirtualNetworks(_ context.Context, _ string) ([]*armnetwork.VirtualNetwork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.VNets, nil
}

func (f *FakeAzureAPI) ListSubnets(_ context.Context, _ string, vnetName string) ([]*armnetwork.Subnet, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Subnets[vnetName], nil
}

func (f *FakeAzureAPI) DeallocateVM(_ context.Context, _ string, vmName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeallocatedVMs[vmName] = true
	return nil
}

func (f *FakeAzureAPI) GetVMInstanceView(_ context.Context, _ string, vmName string) (*armcompute.VirtualMachineInstanceView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	code := "PowerState/running"
	if f.DeallocatedVMs[vmName] {
		code = "PowerState/deallocated"
	}
	return &armcompute.VirtualMachineInstanceView{
		Statuses: []*armcompute.InstanceViewStatus{
			{Code: to.Ptr(code)},
		},
	}, nil
}

func (f *FakeAzureAPI) ListVMSizes(_ context.Context, _ string) ([]*armcompute.VirtualMachineSize, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.VMSizes, nil
}

func (f *FakeAzureAPI) CreateSnapshot(_ context.Context, rg, snapName string, snap armcompute.Snapshot) (*armcompute.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	snap.ID = to.Ptr(fmt.Sprintf("/subscriptions/sub/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s", rg, snapName))
	snap.Name = to.Ptr(snapName)
	provState := "Succeeded"
	snap.Properties = &armcompute.SnapshotProperties{
		ProvisioningState: &provState,
	}
	f.Snapshots[snapName] = &snap
	return &snap, nil
}

func (f *FakeAzureAPI) GetSnapshot(_ context.Context, _, snapName string) (*armcompute.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	snap, ok := f.Snapshots[snapName]
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapName)
	}
	return snap, nil
}

func (f *FakeAzureAPI) DeleteSnapshot(_ context.Context, _, snapName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Snapshots, snapName)
	return nil
}
