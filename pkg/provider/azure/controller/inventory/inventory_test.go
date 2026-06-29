package inventory

import (
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
	web "github.com/kubev2v/forklift/pkg/provider/azure/inventory/web"
)

func TestGetAzureVM_Success(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID:   to.Ptr("/vms/test"),
			Name: to.Ptr("test"),
		},
		ID:   "/vms/test",
		Name: "test",
	}
	inv := &fakeInv{vms: map[string]*model.VMDetails{"vm1": vm}}

	result, err := GetAzureVM(inv, ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("name = %q, want %q", result.Name, "test")
	}
}

func TestGetAzureVM_NotFound(t *testing.T) {
	inv := &fakeInv{vms: map[string]*model.VMDetails{}}

	_, err := GetAzureVM(inv, ref.Ref{ID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent VM")
	}
}

func TestGetAzureVM_NilObject(t *testing.T) {
	inv := &fakeInv{vms: map[string]*model.VMDetails{"vm1": nil}}

	_, err := GetAzureVM(inv, ref.Ref{ID: "vm1"})
	if err == nil {
		t.Fatal("expected error for nil Object")
	}
	if !errors.Is(err, ErrNoAzureVMObject) {
		t.Errorf("expected ErrNoAzureVMObject, got %v", err)
	}
}

func TestGetAzureVM_RestoresDisks(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID: to.Ptr("/vms/test"),
		},
		Name: "test",
	}
	webVM := &web.VM{
		Object: vm,
		Disks: []model.VMDisk{
			{ID: "/d1", Name: "disk1", SizeGB: 64},
		},
	}
	inv := &fakeInvWithWebVM{webVMs: map[string]*web.VM{"vm1": webVM}}

	result, err := GetAzureVM(inv, ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(result.Disks))
	}
	if result.Disks[0].Name != "disk1" {
		t.Errorf("disk name = %q, want %q", result.Disks[0].Name, "disk1")
	}
}

func TestGetManagedDisks_PrefersManaged(t *testing.T) {
	vm := &model.VMDetails{
		Disks:        []model.VMDisk{{Name: "regular"}},
		ManagedDisks: []model.VMDisk{{Name: "managed"}},
	}
	disks := GetManagedDisks(vm)
	if len(disks) != 1 || disks[0].Name != "managed" {
		t.Errorf("expected ManagedDisks to take priority, got %v", disks)
	}
}

func TestGetManagedDisks_FallsBackToDisks(t *testing.T) {
	vm := &model.VMDetails{
		Disks: []model.VMDisk{{Name: "regular"}},
	}
	disks := GetManagedDisks(vm)
	if len(disks) != 1 || disks[0].Name != "regular" {
		t.Errorf("expected fallback to Disks, got %v", disks)
	}
}

func TestGetManagedDiskIDs(t *testing.T) {
	vm := &model.VMDetails{
		Disks: []model.VMDisk{
			{ID: "/d1", Name: "disk1"},
			{ID: "", Name: "disk-no-id"},
			{ID: "/d3", Name: "disk3"},
		},
	}
	ids := GetManagedDiskIDs(vm)
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs (skipping empty), got %d", len(ids))
	}
	if ids[0] != "/d1" || ids[1] != "/d3" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestGetNetworkInterfaces(t *testing.T) {
	vm := &model.VMDetails{
		NetworkInterfaces: []model.VMNetworkInterface{
			{ID: "/nic1"},
			{ID: "/nic2"},
		},
	}
	nics, ok := GetNetworkInterfaces(vm)
	if !ok {
		t.Error("expected ok=true")
	}
	if len(nics) != 2 {
		t.Errorf("expected 2 NICs, got %d", len(nics))
	}
}

func TestGetNetworkInterfaces_Empty(t *testing.T) {
	vm := &model.VMDetails{}
	_, ok := GetNetworkInterfaces(vm)
	if ok {
		t.Error("expected ok=false for no NICs")
	}
}

func TestGetVMName(t *testing.T) {
	tests := []struct {
		name     string
		vm       *model.VMDetails
		expected string
	}{
		{"uses Name when set", &model.VMDetails{Name: "my-vm", ID: "id-123"}, "my-vm"},
		{"falls back to ID", &model.VMDetails{ID: "id-123"}, "id-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetVMName(tt.vm)
			if got != tt.expected {
				t.Errorf("GetVMName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// fakeInv implements Inventory interface for basic testing.
type fakeInv struct {
	vms map[string]*model.VMDetails
}

func (f *fakeInv) Find(resource interface{}, r ref.Ref) error {
	switch res := resource.(type) {
	case *web.VM:
		id := r.ID
		if id == "" {
			id = r.Name
		}
		vm, ok := f.vms[id]
		if !ok {
			return errors.New("not found")
		}
		res.Object = vm
		return nil
	}
	return errors.New("unknown resource type")
}

// fakeInvWithWebVM allows setting the full web.VM including Disks.
type fakeInvWithWebVM struct {
	webVMs map[string]*web.VM
}

func (f *fakeInvWithWebVM) Find(resource interface{}, r ref.Ref) error {
	switch res := resource.(type) {
	case *web.VM:
		id := r.ID
		if id == "" {
			id = r.Name
		}
		webVM, ok := f.webVMs[id]
		if !ok {
			return errors.New("not found")
		}
		*res = *webVM
		return nil
	}
	return errors.New("unknown resource type")
}
