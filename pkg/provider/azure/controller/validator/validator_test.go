package validator

import (
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
	web "github.com/kubev2v/forklift/pkg/provider/azure/inventory/web"
	"github.com/kubev2v/forklift/pkg/provider/testutil"
)

func newTestValidator(vms map[string]*model.VMDetails, storageMap *api.StorageMap, networkMap *api.NetworkMap) *Validator {
	ctx := testutil.NewContextBuilder().
		WithStorageMap(storageMap).
		WithNetworkMap(networkMap).
		Build()
	ctx.Source.Inventory = &fakeInventory{vms: vms}
	return New(ctx)
}

func TestValidateStorage_WithManagedDisks(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						ManagedDisk: &armcompute.ManagedDiskParameters{
							ID: to.Ptr("/disks/os"),
						},
					},
				},
			},
		},
		Disks: []model.VMDisk{
			{ID: "/disks/os", Name: "os", SizeGB: 128},
		},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, nil, nil)
	ok, err := v.validateStorage(ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected storage validation to pass")
	}
}

func TestValidateStorage_NoDisks(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk:    &armcompute.OSDisk{},
					DataDisks: []*armcompute.DataDisk{},
				},
			},
		},
		Disks: []model.VMDisk{},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, nil, nil)
	ok, err := v.validateStorage(ref.Ref{ID: "vm1"})
	if ok {
		t.Error("expected storage validation to fail for VM with no disks")
	}
	if err == nil {
		t.Error("expected error for VM with no disks")
	}
}

func TestStorageMapped_AllMapped(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
					DataDisks: []*armcompute.DataDisk{
						{
							ManagedDisk: &armcompute.ManagedDiskParameters{
								StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
							},
						},
					},
				},
			},
		},
	}

	storageMap := &api.StorageMap{
		Spec: api.StorageMapSpec{
			Map: []api.StoragePair{
				{Source: ref.Ref{Name: "Premium_LRS"}, Destination: api.DestinationStorage{StorageClass: "sc1"}},
				{Source: ref.Ref{Name: "Standard_LRS"}, Destination: api.DestinationStorage{StorageClass: "sc2"}},
			},
		},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, storageMap, nil)
	ok, err := v.StorageMapped(ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected StorageMapped=true when all SKUs are mapped")
	}
}

func TestStorageMapped_UnmappedSKU(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
					DataDisks: []*armcompute.DataDisk{},
				},
			},
		},
	}

	storageMap := &api.StorageMap{
		Spec: api.StorageMapSpec{
			Map: []api.StoragePair{
				{Source: ref.Ref{Name: "Standard_LRS"}, Destination: api.DestinationStorage{StorageClass: "sc"}},
			},
		},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, storageMap, nil)
	ok, err := v.StorageMapped(ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected StorageMapped=false when OS disk SKU is not mapped")
	}
}

func TestUnSupportedDisks_Ephemeral(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						DiffDiskSettings: &armcompute.DiffDiskSettings{
							Option: to.Ptr(armcompute.DiffDiskOptionsLocal),
						},
					},
					DataDisks: []*armcompute.DataDisk{},
				},
			},
		},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, nil, nil)
	unsupported, err := v.UnSupportedDisks(ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported disk, got %d", len(unsupported))
	}
	if unsupported[0] != "OS disk (ephemeral)" {
		t.Errorf("unexpected unsupported disk: %s", unsupported[0])
	}
}

func TestUnSupportedDisks_UnmanagedVHD(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/d")},
					},
					DataDisks: []*armcompute.DataDisk{
						{
							Name: to.Ptr("unmanaged-data"),
							Vhd:  &armcompute.VirtualHardDisk{URI: to.Ptr("https://storage.blob.core.windows.net/vhds/data.vhd")},
						},
					},
				},
			},
		},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, nil, nil)
	unsupported, err := v.UnSupportedDisks(ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported disk, got %d", len(unsupported))
	}
	if unsupported[0] != "unmanaged-data (unmanaged VHD)" {
		t.Errorf("unexpected unsupported disk: %s", unsupported[0])
	}
}

func TestUnSupportedDisks_AllManaged(t *testing.T) {
	vm := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			Properties: &armcompute.VirtualMachineProperties{
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/d")},
					},
					DataDisks: []*armcompute.DataDisk{
						{
							Name:        to.Ptr("data-1"),
							ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/d1")},
						},
					},
				},
			},
		},
	}

	v := newTestValidator(map[string]*model.VMDetails{"vm1": vm}, nil, nil)
	unsupported, err := v.UnSupportedDisks(ref.Ref{ID: "vm1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(unsupported) != 0 {
		t.Errorf("expected 0 unsupported disks, got %v", unsupported)
	}
}

// fakeInventory implements base.Client for tests.
type fakeInventory struct {
	vms map[string]*model.VMDetails
}

var _ base.Client = (*fakeInventory)(nil)

func (f *fakeInventory) Finder() base.Finder { return nil }

func (f *fakeInventory) Get(resource interface{}, id string) error {
	return errors.New("not implemented")
}

func (f *fakeInventory) List(list interface{}, param ...base.Param) error {
	return errors.New("not implemented")
}

func (f *fakeInventory) Watch(resource interface{}, h base.EventHandler) (*base.Watch, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeInventory) Find(resource interface{}, r ref.Ref) error {
	switch res := resource.(type) {
	case *web.VM:
		id := r.ID
		if id == "" {
			id = r.Name
		}
		if vm, ok := f.vms[id]; ok {
			res.Object = vm
			res.Disks = vm.Disks
			return nil
		}
		return errors.New("VM not found")
	}
	return errors.New("unknown resource type")
}

func (f *fakeInventory) VM(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeInventory) Workload(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeInventory) Network(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeInventory) Storage(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeInventory) Host(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}
