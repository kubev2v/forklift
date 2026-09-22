package client

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/kubev2v/forklift/pkg/provider/azure/testutil"
)

func TestClientSubscriptionScoped(t *testing.T) {
	fake := testutil.NewFakeAzureAPI()
	fake.VMs = []*armcompute.VirtualMachine{
		testutil.NewTestVM("vm-1", "Standard_D2s_v3"),
	}

	c := NewWithClient(fake, "sub-1", "")
	if !c.IsSubscriptionScoped() {
		t.Fatal("expected client to be subscription scoped")
	}

	vms, err := c.ListVirtualMachines(context.Background())
	if err != nil {
		t.Fatalf("ListVirtualMachines() error = %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("ListVirtualMachines() count = %d, want 1", len(vms))
	}
}

func TestClientResourceGroupScoped(t *testing.T) {
	fake := testutil.NewFakeAzureAPI()
	fake.VMs = []*armcompute.VirtualMachine{
		testutil.NewTestVM("vm-1", "Standard_D2s_v3"),
	}

	c := NewWithClient(fake, "sub-1", "my-rg")
	if c.IsSubscriptionScoped() {
		t.Fatal("expected client to be resource group scoped")
	}

	vms, err := c.ListVirtualMachines(context.Background())
	if err != nil {
		t.Fatalf("ListVirtualMachines() error = %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("ListVirtualMachines() count = %d, want 1", len(vms))
	}
}

func TestClientGetVMInstanceViewUsesProvidedResourceGroup(t *testing.T) {
	fake := testutil.NewFakeAzureAPI()
	c := NewWithClient(fake, "sub-1", "")

	_, err := c.GetVMInstanceView(context.Background(), "other-rg", "vm-1")
	if err != nil {
		t.Fatalf("GetVMInstanceView() error = %v", err)
	}
}

func TestClientListSubnetsUsesProvidedResourceGroup(t *testing.T) {
	fake := testutil.NewFakeAzureAPI()
	fake.Subnets["vnet-1"] = []*armnetwork.Subnet{
		{ID: to.Ptr("/subscriptions/sub/resourceGroups/net-rg/providers/Microsoft.Network/virtualNetworks/vnet-1/subnets/subnet-1")},
	}
	c := NewWithClient(fake, "sub-1", "")

	subnets, err := c.ListSubnets(context.Background(), "net-rg", "vnet-1")
	if err != nil {
		t.Fatalf("ListSubnets() error = %v", err)
	}
	if len(subnets) != 1 {
		t.Fatalf("ListSubnets() count = %d, want 1", len(subnets))
	}
}
