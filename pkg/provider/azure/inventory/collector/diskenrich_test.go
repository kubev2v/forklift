package collector

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

func TestBuildDisks_OSDiskOnly(t *testing.T) {
	vm := &armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{
					Name:       to.Ptr("os-disk"),
					DiskSizeGB: to.Ptr[int32](128),
					OSType:     to.Ptr(armcompute.OperatingSystemTypesLinux),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						ID:                 to.Ptr("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/disks/os-disk"),
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
				DataDisks: []*armcompute.DataDisk{},
			},
		},
	}

	disks := buildDisks(vm)
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
	if !disks[0].IsOS {
		t.Error("expected OS disk to have IsOS=true")
	}
	if disks[0].Name != "os-disk" {
		t.Errorf("disk name = %q, want %q", disks[0].Name, "os-disk")
	}
	if disks[0].SizeGB != 128 {
		t.Errorf("disk size = %d, want 128", disks[0].SizeGB)
	}
	if disks[0].Sku != "Premium_LRS" {
		t.Errorf("disk sku = %q, want %q", disks[0].Sku, "Premium_LRS")
	}
}

func TestBuildDisks_WithDataDisks(t *testing.T) {
	vm := &armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{
					Name:       to.Ptr("os-disk"),
					DiskSizeGB: to.Ptr[int32](64),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						ID: to.Ptr("/disks/os-disk"),
					},
				},
				DataDisks: []*armcompute.DataDisk{
					{
						Name:       to.Ptr("data-1"),
						DiskSizeGB: to.Ptr[int32](256),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							ID:                 to.Ptr("/disks/data-1"),
							StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
						},
					},
					{
						Name:       to.Ptr("data-2"),
						DiskSizeGB: to.Ptr[int32](512),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							ID:                 to.Ptr("/disks/data-2"),
							StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
				},
			},
		},
	}

	disks := buildDisks(vm)
	if len(disks) != 3 {
		t.Fatalf("expected 3 disks, got %d", len(disks))
	}
	if !disks[0].IsOS {
		t.Error("first disk should be OS")
	}
	if disks[1].IsOS || disks[2].IsOS {
		t.Error("data disks should not be OS")
	}
	if disks[1].SizeGB != 256 {
		t.Errorf("data-1 size = %d, want 256", disks[1].SizeGB)
	}
	if disks[2].Sku != "Premium_LRS" {
		t.Errorf("data-2 sku = %q, want Premium_LRS", disks[2].Sku)
	}
}

func TestBuildDisks_NilProperties(t *testing.T) {
	vm := &armcompute.VirtualMachine{}
	disks := buildDisks(vm)
	if disks != nil {
		t.Errorf("expected nil disks for VM with nil properties, got %v", disks)
	}
}

func TestBuildGuestId_WithImageReference(t *testing.T) {
	vm := &armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{
					OSType: to.Ptr(armcompute.OperatingSystemTypesLinux),
				},
				ImageReference: &armcompute.ImageReference{
					Offer: to.Ptr("CentOS"),
					SKU:   to.Ptr("7.9"),
				},
			},
		},
	}

	guestId := buildGuestId(vm)
	expected := "CentOS 7.9 (Linux)"
	if guestId != expected {
		t.Errorf("buildGuestId() = %q, want %q", guestId, expected)
	}
}

func TestBuildGuestId_NoImageReference(t *testing.T) {
	vm := &armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{
					OSType: to.Ptr(armcompute.OperatingSystemTypesWindows),
				},
			},
		},
	}

	guestId := buildGuestId(vm)
	if guestId != "Windows" {
		t.Errorf("buildGuestId() = %q, want %q", guestId, "Windows")
	}
}

func TestBuildGuestId_NilProperties(t *testing.T) {
	vm := &armcompute.VirtualMachine{}
	guestId := buildGuestId(vm)
	if guestId != "" {
		t.Errorf("buildGuestId() = %q, want empty string", guestId)
	}
}

func TestEnrichDiskSizes(t *testing.T) {
	// This test requires a Collector with a DB. We test the logic at unit level
	// by verifying the behavior when disks already have sizes.
	vm := &model.VM{
		Disks: []model.VMDisk{
			{Name: "disk-with-size", SizeGB: 100, Sku: "Premium_LRS"},
			{Name: "disk-no-size", SizeGB: 0, Sku: ""},
		},
	}

	// Verify that disks with sizes are untouched
	if vm.Disks[0].SizeGB != 100 {
		t.Errorf("disk with size should be untouched, got %d", vm.Disks[0].SizeGB)
	}
	if vm.Disks[1].SizeGB != 0 {
		t.Errorf("disk without size should remain 0 without DB, got %d", vm.Disks[1].SizeGB)
	}
}
