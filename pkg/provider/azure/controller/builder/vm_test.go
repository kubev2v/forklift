package builder

import (
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/azure"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
	web "github.com/kubev2v/forklift/pkg/provider/azure/inventory/web"
	"github.com/kubev2v/forklift/pkg/provider/testutil"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
)

func newFakeBuilder(vms map[string]*model.VMDetails) *Builder {
	networkMap := &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{},
		},
	}
	ctx := testutil.NewContextBuilder().
		WithNetworkMap(networkMap).
		Build()
	ctx.Source.Inventory = &fakeInventory{vms: vms}
	return New(ctx)
}

func TestVirtualMachine_BasicBuild(t *testing.T) {
	vmDetails := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID:   to.Ptr("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/test-vm"),
			Name: to.Ptr("test-vm"),
			Properties: &armcompute.VirtualMachineProperties{
				HardwareProfile: &armcompute.HardwareProfile{
					VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_D2s_v3")),
				},
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						Name:       to.Ptr("os-disk"),
						DiskSizeGB: to.Ptr[int32](128),
						OSType:     to.Ptr(armcompute.OperatingSystemTypesLinux),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							ID: to.Ptr("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/disks/os-disk"),
						},
					},
					DataDisks: []*armcompute.DataDisk{},
				},
				NetworkProfile: &armcompute.NetworkProfile{
					NetworkInterfaces: []*armcompute.NetworkInterfaceReference{},
				},
			},
		},
		ID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/test-vm",
		Name: "test-vm",
	}

	builder := newFakeBuilder(map[string]*model.VMDetails{
		"vm-123": vmDetails,
	})
	vmRef := ref.Ref{ID: "vm-123"}

	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name: "pvc-0",
			Labels: map[string]string{
				azure.LabelDiskIndex: "0",
			},
		},
	}

	vmSpec := &cnv.VirtualMachineSpec{}
	err := builder.VirtualMachine(vmRef, vmSpec, []*core.PersistentVolumeClaim{pvc}, false, false)
	if err != nil {
		t.Fatalf("VirtualMachine() error: %v", err)
	}

	if vmSpec.Template == nil {
		t.Fatal("expected Template to be set")
	}

	cpu := vmSpec.Template.Spec.Domain.CPU
	if cpu == nil {
		t.Fatal("expected CPU to be set")
	}
	if cpu.Cores != 2 {
		t.Errorf("CPU cores = %d, want 2", cpu.Cores)
	}

	mem := vmSpec.Template.Spec.Domain.Resources.Requests[core.ResourceMemory]
	expectedMemBytes := int64(8192) * 1024 * 1024
	if mem.Value() != expectedMemBytes {
		t.Errorf("Memory = %d bytes, want %d bytes (8192Mi)", mem.Value(), expectedMemBytes)
	}

	if len(vmSpec.Template.Spec.Domain.Devices.Disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(vmSpec.Template.Spec.Domain.Devices.Disks))
	}
	if vmSpec.Template.Spec.Domain.Devices.Disks[0].BootOrder == nil {
		t.Error("expected OS disk to have boot order")
	}

	fw := vmSpec.Template.Spec.Domain.Firmware
	if fw == nil {
		t.Fatal("expected Firmware to be set")
	}
	if fw.Bootloader.BIOS == nil {
		t.Error("expected BIOS bootloader for Gen1 VM")
	}
}

func TestVirtualMachine_Gen2UEFI(t *testing.T) {
	vmDetails := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID:   to.Ptr("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/gen2-vm"),
			Name: to.Ptr("gen2-vm"),
			Properties: &armcompute.VirtualMachineProperties{
				HardwareProfile: &armcompute.HardwareProfile{
					VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_B1s")),
				},
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						Name: to.Ptr("os-disk"),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							ID: to.Ptr("/disks/os"),
						},
					},
					DataDisks: []*armcompute.DataDisk{},
				},
				SecurityProfile: &armcompute.SecurityProfile{
					UefiSettings: &armcompute.UefiSettings{
						SecureBootEnabled: to.Ptr(true),
					},
				},
				NetworkProfile: &armcompute.NetworkProfile{
					NetworkInterfaces: []*armcompute.NetworkInterfaceReference{},
				},
			},
		},
		ID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/gen2-vm",
		Name: "gen2-vm",
	}

	builder := newFakeBuilder(map[string]*model.VMDetails{"gen2": vmDetails})
	vmRef := ref.Ref{ID: "gen2"}

	vmSpec := &cnv.VirtualMachineSpec{}
	err := builder.VirtualMachine(vmRef, vmSpec, []*core.PersistentVolumeClaim{}, false, false)
	if err != nil {
		t.Fatalf("VirtualMachine() error: %v", err)
	}

	fw := vmSpec.Template.Spec.Domain.Firmware
	if fw == nil || fw.Bootloader == nil {
		t.Fatal("expected firmware with bootloader")
	}
	if fw.Bootloader.EFI == nil {
		t.Error("expected EFI bootloader for Gen2 VM")
	}
	if vmSpec.Template.Spec.Domain.Features == nil || vmSpec.Template.Spec.Domain.Features.SMM == nil {
		t.Error("expected SMM feature for Gen2 VM")
	}
}

func TestVirtualMachine_RunStrategyPreserved(t *testing.T) {
	vmDetails := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID:   to.Ptr("/vms/vm1"),
			Name: to.Ptr("vm1"),
			Properties: &armcompute.VirtualMachineProperties{
				HardwareProfile: &armcompute.HardwareProfile{
					VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_B1s")),
				},
				StorageProfile: &armcompute.StorageProfile{
					OSDisk:    &armcompute.OSDisk{ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/d")}},
					DataDisks: []*armcompute.DataDisk{},
				},
				NetworkProfile: &armcompute.NetworkProfile{
					NetworkInterfaces: []*armcompute.NetworkInterfaceReference{},
				},
			},
		},
		ID:   "/vms/vm1",
		Name: "vm1",
	}

	builder := newFakeBuilder(map[string]*model.VMDetails{"v1": vmDetails})
	vmRef := ref.Ref{ID: "v1"}

	strategy := cnv.RunStrategyAlways
	vmSpec := &cnv.VirtualMachineSpec{
		RunStrategy: &strategy,
	}
	err := builder.VirtualMachine(vmRef, vmSpec, []*core.PersistentVolumeClaim{}, false, false)
	if err != nil {
		t.Fatalf("VirtualMachine() error: %v", err)
	}

	if vmSpec.RunStrategy == nil {
		t.Fatal("RunStrategy was cleared")
	}
	if *vmSpec.RunStrategy != cnv.RunStrategyAlways {
		t.Errorf("RunStrategy = %v, want Always", *vmSpec.RunStrategy)
	}
}

func TestVirtualMachine_DefaultNetworkWithPod(t *testing.T) {
	vmDetails := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID:   to.Ptr("/vms/vm1"),
			Name: to.Ptr("vm1"),
			Properties: &armcompute.VirtualMachineProperties{
				HardwareProfile: &armcompute.HardwareProfile{
					VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_B1s")),
				},
				StorageProfile: &armcompute.StorageProfile{
					OSDisk:    &armcompute.OSDisk{ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/d")}},
					DataDisks: []*armcompute.DataDisk{},
				},
				NetworkProfile: &armcompute.NetworkProfile{
					NetworkInterfaces: []*armcompute.NetworkInterfaceReference{},
				},
			},
		},
		ID:   "/vms/vm1",
		Name: "vm1",
	}

	builder := newFakeBuilder(map[string]*model.VMDetails{"v1": vmDetails})
	vmRef := ref.Ref{ID: "v1"}

	vmSpec := &cnv.VirtualMachineSpec{}
	err := builder.VirtualMachine(vmRef, vmSpec, []*core.PersistentVolumeClaim{}, false, false)
	if err != nil {
		t.Fatalf("VirtualMachine() error: %v", err)
	}

	nets := vmSpec.Template.Spec.Networks
	if len(nets) != 1 {
		t.Fatalf("expected 1 network, got %d", len(nets))
	}
	if nets[0].Pod == nil {
		t.Error("expected Pod network for VM with no NICs")
	}

	ifaces := vmSpec.Template.Spec.Domain.Devices.Interfaces
	if len(ifaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(ifaces))
	}
	if ifaces[0].Masquerade == nil {
		t.Error("expected Masquerade binding")
	}
}

func TestVirtualMachine_MultipleDataDisks(t *testing.T) {
	vmDetails := &model.VMDetails{
		VirtualMachine: armcompute.VirtualMachine{
			ID:   to.Ptr("/vms/multi-disk"),
			Name: to.Ptr("multi-disk"),
			Properties: &armcompute.VirtualMachineProperties{
				HardwareProfile: &armcompute.HardwareProfile{
					VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_D4s_v3")),
				},
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{
						Name:        to.Ptr("os-disk"),
						ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/disks/os")},
					},
					DataDisks: []*armcompute.DataDisk{
						{Name: to.Ptr("data-0"), Lun: to.Ptr[int32](0), ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/disks/d0")}},
						{Name: to.Ptr("data-1"), Lun: to.Ptr[int32](1), ManagedDisk: &armcompute.ManagedDiskParameters{ID: to.Ptr("/disks/d1")}},
					},
				},
				NetworkProfile: &armcompute.NetworkProfile{
					NetworkInterfaces: []*armcompute.NetworkInterfaceReference{},
				},
			},
		},
		ID:   "/vms/multi-disk",
		Name: "multi-disk",
	}

	builder := newFakeBuilder(map[string]*model.VMDetails{"md": vmDetails})
	vmRef := ref.Ref{ID: "md"}

	pvcs := []*core.PersistentVolumeClaim{
		{ObjectMeta: meta.ObjectMeta{Name: "pvc-0", Labels: map[string]string{azure.LabelDiskIndex: "0"}}},
		{ObjectMeta: meta.ObjectMeta{Name: "pvc-1", Labels: map[string]string{azure.LabelDiskIndex: "1"}}},
		{ObjectMeta: meta.ObjectMeta{Name: "pvc-2", Labels: map[string]string{azure.LabelDiskIndex: "2"}}},
	}

	vmSpec := &cnv.VirtualMachineSpec{}
	err := builder.VirtualMachine(vmRef, vmSpec, pvcs, false, false)
	if err != nil {
		t.Fatalf("VirtualMachine() error: %v", err)
	}

	disks := vmSpec.Template.Spec.Domain.Devices.Disks
	if len(disks) != 3 {
		t.Fatalf("expected 3 disks (1 OS + 2 data), got %d", len(disks))
	}

	if disks[0].BootOrder == nil {
		t.Error("OS disk should have boot order")
	}
	for i := 1; i < len(disks); i++ {
		if disks[i].BootOrder != nil {
			t.Errorf("data disk %d should not have boot order", i)
		}
	}

	volumes := vmSpec.Template.Spec.Volumes
	if len(volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(volumes))
	}
	for i, vol := range volumes {
		expected := pvcs[i].Name
		if vol.PersistentVolumeClaim == nil {
			t.Errorf("volume %d missing PVC source", i)
			continue
		}
		if vol.PersistentVolumeClaim.ClaimName != expected {
			t.Errorf("volume %d claim = %q, want %q", i, vol.PersistentVolumeClaim.ClaimName, expected)
		}
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
