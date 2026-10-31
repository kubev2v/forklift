package vsphere

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/vmware/govmomi/vim25/types"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- helpers for building packed PCI slot numbers ---
func makeSlot(dev, bridge, fn int32) int32 {
	return (dev & pciSlotMaskDevice) |
		((bridge & pciSlotMaskBridge) << pciSlotShiftBridge) |
		((fn & pciSlotMaskFunc) << pciSlotShiftFunc)
}

// --- helpers for building govmomi virtual devices ---
func makeVmxnet3(key int32, slot int32, backing types.BaseVirtualDeviceBackingInfo, mac string) *types.VirtualVmxnet3 {
	return &types.VirtualVmxnet3{
		VirtualVmxnet: types.VirtualVmxnet{
			VirtualEthernetCard: types.VirtualEthernetCard{
				VirtualDevice: types.VirtualDevice{
					Key:      key,
					SlotInfo: &types.VirtualDevicePciBusSlotInfo{PciSlotNumber: slot},
					Backing:  backing,
				},
				MacAddress: mac,
			},
		},
	}
}

func makeE1000(key int32, backing types.BaseVirtualDeviceBackingInfo) *types.VirtualE1000 {
	return &types.VirtualE1000{
		VirtualEthernetCard: types.VirtualEthernetCard{
			VirtualDevice: types.VirtualDevice{Key: key, Backing: backing},
		},
	}
}

func makeSriov(key int32, backing types.BaseVirtualDeviceBackingInfo) *types.VirtualSriovEthernetCard {
	return &types.VirtualSriovEthernetCard{
		VirtualEthernetCard: types.VirtualEthernetCard{
			VirtualDevice: types.VirtualDevice{Key: key, Backing: backing},
		},
	}
}

func makePCIPassthrough(key int32) *types.VirtualPCIPassthrough {
	return &types.VirtualPCIPassthrough{
		VirtualDevice: types.VirtualDevice{Key: key},
	}
}

func makeUSBController(key int32) *types.VirtualUSBController {
	return &types.VirtualUSBController{
		VirtualController: types.VirtualController{
			VirtualDevice: types.VirtualDevice{Key: key},
		},
	}
}

func makeDisk(key int32) *types.VirtualDisk {
	return &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{Key: key},
	}
}

func netBacking(networkID string) *types.VirtualEthernetCardNetworkBackingInfo {
	return &types.VirtualEthernetCardNetworkBackingInfo{
		Network: &types.ManagedObjectReference{Value: networkID},
	}
}

// --- computePciAddress tests ---

func TestComputePciAddress(t *testing.T) {
	bridges := []model.PciBridge{
		{Number: 0, SlotNumber: 17, Functions: 1},
		{Number: 4, SlotNumber: 21, Functions: 8},
		{Number: 5, SlotNumber: 22, Functions: 8},
	}

	tests := []struct {
		name     string
		slot     int32
		bridges  []model.PciBridge
		expected string
	}{
		{
			name:     "zero slot returns empty",
			slot:     0,
			bridges:  bridges,
			expected: "",
		},
		{
			name:     "negative slot returns empty",
			slot:     -1,
			bridges:  bridges,
			expected: "",
		},
		{
			name:     "primary bus device (bridgeIdx=0)",
			slot:     makeSlot(0x11, 0, 0),
			bridges:  bridges,
			expected: "0000:00:11.0",
		},
		{
			name:     "primary bus device with empty bridges",
			slot:     makeSlot(0x11, 0, 0),
			bridges:  nil,
			expected: "0000:00:11.0",
		},
		{
			name:     "device on pciBridge0 (bridgeIdx=1, bridgeNum=0)",
			slot:     makeSlot(0x00, 1, 0),
			bridges:  bridges,
			expected: "0000:02:00.0",
		},
		{
			name:     "device on pciBridge4 func 0 (bridgeIdx=5, bridgeNum=4)",
			slot:     makeSlot(0x00, 5, 0),
			bridges:  bridges,
			expected: "0000:03:00.0",
		},
		{
			name:     "device on pciBridge4 func 2",
			slot:     makeSlot(0x00, 5, 2),
			bridges:  bridges,
			expected: "0000:05:00.0",
		},
		{
			name:     "device on pciBridge5 func 0",
			slot:     makeSlot(0x00, 6, 0),
			bridges:  bridges,
			expected: "0000:0b:00.0",
		},
		{
			name:     "non-primary bus with empty bridges returns empty",
			slot:     makeSlot(0x00, 5, 0),
			bridges:  nil,
			expected: "",
		},
		{
			name:     "bridge not found returns empty",
			slot:     makeSlot(0x00, 10, 0),
			bridges:  bridges,
			expected: "",
		},
		{
			name:     "real slot 192 matches test VM NIC",
			slot:     192,
			bridges:  bridges,
			expected: "0000:0b:00.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePciAddress(tt.slot, tt.bridges)
			if got != tt.expected {
				t.Errorf("computePciAddress(%d, bridges) = %q, want %q", tt.slot, got, tt.expected)
			}
		})
	}
}

// --- isPassthroughOrController tests ---

func TestIsPassthroughOrController(t *testing.T) {
	tests := []struct {
		name     string
		dev      types.BaseVirtualDevice
		expected bool
	}{
		{"VirtualSriovEthernetCard", makeSriov(1, nil), true},
		{"VirtualPCIPassthrough", makePCIPassthrough(2), true},
		{"VirtualUSBController", makeUSBController(3), true},
		{"VirtualVmxnet3", makeVmxnet3(4, 0, netBacking("net-1"), ""), false},
		{"VirtualE1000", makeE1000(5, netBacking("net-1")), false},
		{"VirtualDisk", makeDisk(6), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPassthroughOrController(tt.dev)
			if got != tt.expected {
				t.Errorf("isPassthroughOrController(%s) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// --- isConnectedNIC tests ---

func TestIsConnectedNIC(t *testing.T) {
	tests := []struct {
		name     string
		dev      types.BaseVirtualDevice
		expected bool
	}{
		{"vmxnet3 with backing", makeVmxnet3(1, 0, netBacking("net-1"), ""), true},
		{"e1000 with backing", makeE1000(2, netBacking("net-1")), true},
		{"vmxnet3 nil backing", makeVmxnet3(3, 0, nil, ""), false},
		{"SR-IOV with backing", makeSriov(4, netBacking("net-1")), false},
		{"SR-IOV nil backing", makeSriov(5, nil), false},
		{"PCI passthrough", makePCIPassthrough(6), false},
		{"USB controller", makeUSBController(7), false},
		{"virtual disk", makeDisk(8), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConnectedNIC(tt.dev)
			if got != tt.expected {
				t.Errorf("isConnectedNIC(%s) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// --- refreshNICPciAddresses tests ---

func TestRefreshNICPciAddresses(t *testing.T) {
	bridges := []model.PciBridge{
		{Number: 0, SlotNumber: 17, Functions: 1},
		{Number: 4, SlotNumber: 21, Functions: 8},
	}

	vm := &model.VM{
		PciBridges: bridges,
		Devices: []model.Device{
			{Key: 4000, PciSlotNumber: 192},
			{Key: 4001, PciSlotNumber: 224},
		},
		NICs: []model.NIC{
			{DeviceKey: 4000, PciAddress: "stale"},
			{DeviceKey: 4001, PciAddress: "stale"},
		},
	}

	refreshNICPciAddresses(vm)

	expected0 := computePciAddress(192, bridges)
	expected1 := computePciAddress(224, bridges)

	if vm.NICs[0].PciAddress != expected0 {
		t.Errorf("NIC[0].PciAddress = %q, want %q", vm.NICs[0].PciAddress, expected0)
	}
	if vm.NICs[1].PciAddress != expected1 {
		t.Errorf("NIC[1].PciAddress = %q, want %q", vm.NICs[1].PciAddress, expected1)
	}
}

func TestRefreshNICPciAddresses_MissingDevice(t *testing.T) {
	vm := &model.VM{
		PciBridges: []model.PciBridge{{Number: 4, SlotNumber: 21, Functions: 8}},
		Devices:    []model.Device{},
		NICs: []model.NIC{
			{DeviceKey: 9999, PciAddress: "old"},
		},
	}

	refreshNICPciAddresses(vm)

	if vm.NICs[0].PciAddress != "" {
		t.Errorf("NIC with missing device should get empty PciAddress, got %q", vm.NICs[0].PciAddress)
	}
}

// --- collectDevices tests ---

func TestCollectDevices(t *testing.T) {
	v := &VmAdapter{}

	devArray := types.ArrayOfVirtualDevice{
		VirtualDevice: []types.BaseVirtualDevice{
			makeVmxnet3(4000, 192, netBacking("net-1"), "aa:bb:cc:dd:ee:01"),
			makePCIPassthrough(100),
			makeUSBController(200),
			makeDisk(2000),
			makeVmxnet3(4001, 224, nil, "aa:bb:cc:dd:ee:02"),
			makeSriov(300, netBacking("net-2")),
		},
	}

	devList := v.collectDevices(devArray)

	expectedKeys := []int32{4000, 100, 200, 300}
	if len(devList) != len(expectedKeys) {
		t.Fatalf("collectDevices returned %d devices, want %d", len(devList), len(expectedKeys))
	}
	for i, key := range expectedKeys {
		if devList[i].Key != key {
			t.Errorf("devList[%d].Key = %d, want %d", i, devList[i].Key, key)
		}
	}

	if devList[0].PciSlotNumber != 192 {
		t.Errorf("devList[0].PciSlotNumber = %d, want 192", devList[0].PciSlotNumber)
	}
}

func TestCollectDevices_SkipsUnconnectedNIC(t *testing.T) {
	v := &VmAdapter{}

	devArray := types.ArrayOfVirtualDevice{
		VirtualDevice: []types.BaseVirtualDevice{
			makeVmxnet3(4000, 0, nil, ""),
		},
	}

	devList := v.collectDevices(devArray)
	if len(devList) != 0 {
		t.Errorf("collectDevices should skip NIC without backing, got %d devices", len(devList))
	}
}

// --- collectNICs tests ---

func TestCollectNICs(t *testing.T) {
	bridges := []model.PciBridge{
		{Number: 0, SlotNumber: 17, Functions: 1},
		{Number: 4, SlotNumber: 21, Functions: 8},
		{Number: 5, SlotNumber: 22, Functions: 8},
	}

	slot0 := makeSlot(0x00, 5, 0) // on pciBridge4
	slot1 := makeSlot(0x00, 6, 0) // on pciBridge5

	v := &VmAdapter{model: model.VM{PciBridges: bridges}}

	devArray := types.ArrayOfVirtualDevice{
		VirtualDevice: []types.BaseVirtualDevice{
			makeVmxnet3(4000, slot0, netBacking("net-10"), "AA:BB:CC:DD:EE:01"),
			makeVmxnet3(4001, slot1, netBacking("net-20"), "AA:BB:CC:DD:EE:02"),
			makePCIPassthrough(100),
			makeDisk(2000),
			makeSriov(300, netBacking("net-30")),
			makeVmxnet3(4002, 0, nil, "AA:BB:CC:DD:EE:03"),
		},
	}

	nicList := v.collectNICs(devArray)

	if len(nicList) != 2 {
		t.Fatalf("collectNICs returned %d NICs, want 2", len(nicList))
	}

	if nicList[0].MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("NIC[0].MAC = %q, want lowercase", nicList[0].MAC)
	}
	if nicList[0].DeviceKey != 4000 {
		t.Errorf("NIC[0].DeviceKey = %d, want 4000", nicList[0].DeviceKey)
	}
	if nicList[0].Index != 0 {
		t.Errorf("NIC[0].Index = %d, want 0", nicList[0].Index)
	}
	if nicList[0].Network.ID != "net-10" {
		t.Errorf("NIC[0].Network.ID = %q, want net-10", nicList[0].Network.ID)
	}
	if nicList[0].PciAddress == "" {
		t.Error("NIC[0].PciAddress should not be empty")
	}

	if nicList[1].Index != 1 {
		t.Errorf("NIC[1].Index = %d, want 1", nicList[1].Index)
	}
	if nicList[1].DeviceKey != 4001 {
		t.Errorf("NIC[1].DeviceKey = %d, want 4001", nicList[1].DeviceKey)
	}
}

func TestCollectNICs_ExcludesSRIOV(t *testing.T) {
	v := &VmAdapter{}

	devArray := types.ArrayOfVirtualDevice{
		VirtualDevice: []types.BaseVirtualDevice{
			makeSriov(300, netBacking("net-1")),
		},
	}

	nicList := v.collectNICs(devArray)
	if len(nicList) != 0 {
		t.Errorf("collectNICs should exclude SR-IOV NICs, got %d", len(nicList))
	}
}

// Test getDiskGuestInfo method
func TestVmAdapter_getDiskGuestInfo(t *testing.T) {
	tests := []struct {
		name        string
		guestDisks  []model.DiskMountPoint
		deviceKey   int32
		expected    *model.DiskMountPoint
		expectFound bool
	}{
		{
			name: "returns pointer to correct slice element when key exists",
			guestDisks: []model.DiskMountPoint{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            200,
					DiskPath:       "D:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            300,
					DiskPath:       "/home",
					Capacity:       3000000000,
					FreeSpace:      2000000000,
					FilesystemType: "ext4",
				},
			},
			deviceKey: 200,
			expected: &model.DiskMountPoint{
				Key:            200,
				DiskPath:       "D:\\",
				Capacity:       2000000000,
				FreeSpace:      1500000000,
				FilesystemType: "NTFS",
			},
			expectFound: true,
		},
		{
			name: "returns pointer to first element when first key matches",
			guestDisks: []model.DiskMountPoint{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            200,
					DiskPath:       "D:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
			},
			deviceKey: 100,
			expected: &model.DiskMountPoint{
				Key:            100,
				DiskPath:       "C:\\",
				Capacity:       1000000000,
				FreeSpace:      500000000,
				FilesystemType: "NTFS",
			},
			expectFound: true,
		},
		{
			name: "returns pointer to last element when last key matches",
			guestDisks: []model.DiskMountPoint{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            200,
					DiskPath:       "D:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            300,
					DiskPath:       "/home",
					Capacity:       3000000000,
					FreeSpace:      2000000000,
					FilesystemType: "ext4",
				},
			},
			deviceKey: 300,
			expected: &model.DiskMountPoint{
				Key:            300,
				DiskPath:       "/home",
				Capacity:       3000000000,
				FreeSpace:      2000000000,
				FilesystemType: "ext4",
			},
			expectFound: true,
		},
		{
			name: "returns nil when no matching key is found",
			guestDisks: []model.DiskMountPoint{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            200,
					DiskPath:       "D:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
			},
			deviceKey:   999,
			expected:    nil,
			expectFound: false,
		},
		{
			name:        "returns nil when guest disks list is empty",
			guestDisks:  []model.DiskMountPoint{},
			deviceKey:   100,
			expected:    nil,
			expectFound: false,
		},
		{
			name:        "returns nil when guest disks list is nil",
			guestDisks:  nil,
			deviceKey:   100,
			expected:    nil,
			expectFound: false,
		},
		{
			name: "returns nil when searching for zero key that doesn't exist",
			guestDisks: []model.DiskMountPoint{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
			},
			deviceKey:   0,
			expected:    nil,
			expectFound: false,
		},
		{
			name: "returns pointer when zero key exists",
			guestDisks: []model.DiskMountPoint{
				{
					Key:            0,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            100,
					DiskPath:       "D:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
			},
			deviceKey: 0,
			expected: &model.DiskMountPoint{
				Key:            0,
				DiskPath:       "C:\\",
				Capacity:       1000000000,
				FreeSpace:      500000000,
				FilesystemType: "NTFS",
			},
			expectFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup VmAdapter with test data
			v := &VmAdapter{
				model: model.VM{
					GuestDisks: tt.guestDisks,
				},
			}

			// Call the method under test
			result := v.getDiskGuestInfo(tt.deviceKey)

			// Verify the result
			if tt.expectFound {
				if result == nil {
					t.Errorf("expected to find guest disk with key %d, but got nil", tt.deviceKey)
					return
				}

				// Compare the values (not pointer equality since we're comparing to expected struct)
				if result.Key != tt.expected.Key ||
					result.DiskPath != tt.expected.DiskPath ||
					result.Capacity != tt.expected.Capacity ||
					result.FreeSpace != tt.expected.FreeSpace ||
					result.FilesystemType != tt.expected.FilesystemType {
					t.Errorf("getDiskGuestInfo() returned wrong guest disk data.\nExpected: %+v\nGot: %+v", tt.expected, result)
				}

				// Verify that we got a pointer to the actual slice element
				expectedIndex := -1
				for i, disk := range tt.guestDisks {
					if disk.Key == tt.deviceKey {
						expectedIndex = i
						break
					}
				}
				if expectedIndex >= 0 && result != &v.model.GuestDisks[expectedIndex] {
					t.Errorf("getDiskGuestInfo() should return pointer to slice element at index %d", expectedIndex)
				}
			} else {
				if result != nil {
					t.Errorf("expected nil for key %d, but got %+v", tt.deviceKey, result)
				}
			}
		})
	}
}

func TestVmAdapter_Apply_Annotation(t *testing.T) {
	customValues := []model.CustomFieldValue{{Key: 7, Value: "production"}}
	tests := []struct {
		name     string
		initial  string
		changes  []types.PropertyChange
		expected string
	}{
		{
			name:     "collect Notes",
			changes:  []types.PropertyChange{{Op: Assign, Name: fAnnotation, Val: "First line\nSecond line"}},
			expected: "First line\nSecond line",
		},
		{
			name:     "update Notes",
			initial:  "old",
			changes:  []types.PropertyChange{{Op: Assign, Name: fAnnotation, Val: "new"}},
			expected: "new",
		},
		{
			name:    "clear Notes with empty assignment",
			initial: "old",
			changes: []types.PropertyChange{{Op: Assign, Name: fAnnotation, Val: ""}},
		},
		{
			name:    "clear Notes with property removal",
			initial: "old",
			changes: []types.PropertyChange{{Op: types.PropertyChangeOpRemove, Name: fAnnotation}},
		},
		{
			name:     "preserve Notes on unrelated update",
			initial:  "old",
			changes:  []types.PropertyChange{{Op: Assign, Name: fFirmware, Val: "efi"}},
			expected: "old",
		},
		{
			name:     "keep Notes separate from custom values",
			changes:  []types.PropertyChange{{Op: Assign, Name: fAnnotation, Val: "Notes"}},
			expected: "Notes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &VmAdapter{model: model.VM{
				Annotation:   tt.initial,
				CustomValues: customValues,
			}}
			v.Apply(types.ObjectUpdate{ChangeSet: tt.changes})
			if v.model.Annotation != tt.expected {
				t.Errorf("Annotation = %q, want %q", v.model.Annotation, tt.expected)
			}
			if !reflect.DeepEqual(v.model.CustomValues, customValues) {
				t.Errorf("CustomValues = %+v, want %+v", v.model.CustomValues, customValues)
			}
		})
	}
}

func TestVmAdapter_Apply_CustomFields(t *testing.T) {
	tests := []struct {
		name              string
		preExistingValues []model.CustomFieldValue
		customValueVal    interface{}
		expectedValues    []model.CustomFieldValue
	}{
		{
			name: "populates CustomValues from scratch",
			customValueVal: types.ArrayOfCustomFieldValue{
				CustomFieldValue: []types.BaseCustomFieldValue{
					&types.CustomFieldStringValue{
						CustomFieldValue: types.CustomFieldValue{Key: 100},
						Value:            "alice",
					},
					&types.CustomFieldStringValue{
						CustomFieldValue: types.CustomFieldValue{Key: 200},
						Value:            "production",
					},
				},
			},
			expectedValues: []model.CustomFieldValue{
				{Key: 100, Value: "alice"},
				{Key: 200, Value: "production"},
			},
		},
		{
			name: "clears CustomValues when vSphere reports empty customValue (ArrayOf)",
			preExistingValues: []model.CustomFieldValue{
				{Key: 999, Value: "stale"},
			},
			customValueVal: types.ArrayOfCustomFieldValue{
				CustomFieldValue: []types.BaseCustomFieldValue{},
			},
			expectedValues: []model.CustomFieldValue{},
		},
		{
			name: "clears CustomValues when vSphere reports empty customValue (slice)",
			preExistingValues: []model.CustomFieldValue{
				{Key: 999, Value: "stale"},
			},
			customValueVal: []types.BaseCustomFieldValue{},
			expectedValues: []model.CustomFieldValue{},
		},
		{
			name: "handles customValue as []BaseCustomFieldValue",
			customValueVal: []types.BaseCustomFieldValue{
				&types.CustomFieldStringValue{
					CustomFieldValue: types.CustomFieldValue{Key: 10},
					Value:            "value-from-slice",
				},
			},
			expectedValues: []model.CustomFieldValue{
				{Key: 10, Value: "value-from-slice"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &VmAdapter{
				model: model.VM{
					CustomValues: tt.preExistingValues,
				},
			}

			changeSet := []types.PropertyChange{}
			if tt.customValueVal != nil {
				changeSet = append(changeSet, types.PropertyChange{
					Op:   Assign,
					Name: fCustomValue,
					Val:  tt.customValueVal,
				})
			}

			v.Apply(types.ObjectUpdate{
				ChangeSet: changeSet,
			})

			if tt.expectedValues != nil {
				if !reflect.DeepEqual(v.model.CustomValues, tt.expectedValues) {
					t.Errorf("CustomValues mismatch\ngot:  %+v\nwant: %+v", v.model.CustomValues, tt.expectedValues)
				}
			}
		})
	}
}

func newTestInventoryDB(t *testing.T) libmodel.DB {
	t.Helper()
	db := libmodel.New(filepath.Join(t.TempDir(), "test.db"), &model.CustomFieldDef{})
	if err := db.Open(true); err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(true) })
	return db
}

func newTestCollector() *Collector {
	return &Collector{
		customFieldDefs:   map[int32]model.CustomFieldDef{},
		customFieldDefsMu: &sync.RWMutex{},
	}
}

func TestUpsertCustomFieldDefs(t *testing.T) {
	adapter := newTestCollector()
	db := newTestInventoryDB(t)
	defs := []types.CustomFieldDef{
		{Name: "owner", Key: 100, ManagedObjectType: "VirtualMachine"},
		{Name: "env", Key: 200, ManagedObjectType: ""},
	}
	u := types.ObjectUpdate{
		Obj: types.ManagedObjectReference{Type: "VirtualMachine"},
		ChangeSet: []types.PropertyChange{
			{
				Op:   Assign,
				Name: fAvailableField,
				Val: types.ArrayOfCustomFieldDef{
					CustomFieldDef: defs,
				},
			},
		},
	}

	err := db.With(func(tx *libmodel.Tx) error {
		return adapter.upsertCustomFieldDefs(tx, u)
	})
	if err != nil {
		t.Fatalf("upsertCustomFieldDefs returned error: %v", err)
	}

	list := []model.CustomFieldDef{}
	err = db.List(&list, libmodel.ListOptions{Detail: libmodel.MaxDetail})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != len(defs) {
		t.Fatalf("expected %d defs upserted, got %d", len(defs), len(list))
	}
	byKey := make(map[int32]model.CustomFieldDef, len(list))
	for _, d := range list {
		byKey[d.Key] = d
	}
	for _, f := range defs {
		got, ok := byKey[f.Key]
		if !ok {
			t.Errorf("expected def with key %d to be present", f.Key)
			continue
		}
		expected := model.CustomFieldDef{
			Name:              f.Name,
			Key:               f.Key,
			ManagedObjectType: f.ManagedObjectType,
		}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("def %d mismatch\ngot:  %+v\nwant: %+v", f.Key, got, expected)
		}
	}
}

func TestUpsertCustomFieldDefsSkipsNonVM(t *testing.T) {
	adapter := newTestCollector()
	db := newTestInventoryDB(t)

	err := db.With(func(tx *libmodel.Tx) error {
		return adapter.upsertCustomFieldDefs(tx, types.ObjectUpdate{
			Obj: types.ManagedObjectReference{Type: "HostSystem"},
		})
	})
	if err != nil {
		t.Fatalf("upsertCustomFieldDefs returned unexpected error: %v", err)
	}

	list := []model.CustomFieldDef{}
	err = db.List(&list, libmodel.ListOptions{Detail: libmodel.MaxDetail})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected no defs for non-VM object, got %d", len(list))
	}
}

func TestUpsertCustomFieldDefsCaches(t *testing.T) {
	adapter := newTestCollector()
	db := newTestInventoryDB(t)

	defs := []types.CustomFieldDef{
		{Name: "owner", Key: 100, ManagedObjectType: "VirtualMachine"},
		{Name: "env", Key: 200, ManagedObjectType: ""},
	}
	update := func(defs []types.CustomFieldDef) types.ObjectUpdate {
		return types.ObjectUpdate{
			Obj: types.ManagedObjectReference{Type: "VirtualMachine", Value: "vm-1"},
			ChangeSet: []types.PropertyChange{
				{
					Op:   Assign,
					Name: fAvailableField,
					Val:  types.ArrayOfCustomFieldDef{CustomFieldDef: defs},
				},
			},
		}
	}

	countRows := func() int {
		t.Helper()
		list := []model.CustomFieldDef{}
		if err := db.List(&list, libmodel.ListOptions{Detail: libmodel.MaxDetail}); err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		return len(list)
	}

	if err := db.With(func(tx *libmodel.Tx) error {
		return adapter.upsertCustomFieldDefs(tx, update(defs))
	}); err != nil {
		t.Fatalf("first upsert returned error: %v", err)
	}
	if got := countRows(); got != len(defs) {
		t.Fatalf("expected %d rows after first upsert, got %d", len(defs), got)
	}

	// The same provider-global def set reported by a second VM should be
	// skipped via the in-memory cache (no additional DB writes, no dup rows).
	if err := db.With(func(tx *libmodel.Tx) error {
		return adapter.upsertCustomFieldDefs(tx, update(defs))
	}); err != nil {
		t.Fatalf("repeated upsert returned error: %v", err)
	}
	if got := countRows(); got != len(defs) {
		t.Fatalf("expected %d rows after repeated upsert, got %d", len(defs), got)
	}

	// A changed def set should be reflected in the DB.
	changed := []types.CustomFieldDef{
		{Name: "owner", Key: 100, ManagedObjectType: "VirtualMachine"},
		{Name: "env-renamed", Key: 200, ManagedObjectType: ""},
		{Name: "team", Key: 300, ManagedObjectType: "VirtualMachine"},
	}
	if err := db.With(func(tx *libmodel.Tx) error {
		return adapter.upsertCustomFieldDefs(tx, update(changed))
	}); err != nil {
		t.Fatalf("changed upsert returned error: %v", err)
	}
	if got := countRows(); got != len(changed) {
		t.Fatalf("expected %d rows after changed upsert, got %d", len(changed), got)
	}
}

func TestUpsertCustomFieldDefsRollbackRetriesIdentical(t *testing.T) {
	adapter := newTestCollector()
	db := newTestInventoryDB(t)

	defs := []types.CustomFieldDef{
		{Name: "owner", Key: 100, ManagedObjectType: "VirtualMachine"},
		{Name: "env", Key: 200, ManagedObjectType: ""},
	}
	u := types.ObjectUpdate{
		Obj: types.ManagedObjectReference{Type: "VirtualMachine", Value: "vm-1"},
		ChangeSet: []types.PropertyChange{
			{
				Op:   Assign,
				Name: fAvailableField,
				Val:  types.ArrayOfCustomFieldDef{CustomFieldDef: defs},
			},
		},
	}

	countRows := func() int {
		t.Helper()
		list := []model.CustomFieldDef{}
		if err := db.List(&list, libmodel.ListOptions{Detail: libmodel.MaxDetail}); err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		return len(list)
	}

	// Stage the defs inside a transaction that is then rolled back.
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}
	if err := adapter.upsertCustomFieldDefs(tx, u); err != nil {
		t.Fatalf("upsertCustomFieldDefs returned error: %v", err)
	}
	if err := tx.End(); err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	// The cache must not retain defs from the rolled-back transaction.
	adapter.resetCustomFieldDefs()

	if got := countRows(); got != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", got)
	}

	// Retrying identical definitions after the rollback must reinsert them.
	if err := db.With(func(tx *libmodel.Tx) error {
		return adapter.upsertCustomFieldDefs(tx, u)
	}); err != nil {
		t.Fatalf("retry upsert returned error: %v", err)
	}
	if got := countRows(); got != len(defs) {
		t.Fatalf("expected %d rows after retry, got %d", len(defs), got)
	}
}

func TestHasDiskPrefix(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"scsi0:0.ctkEnabled", true},
		{"SCSI0:0.ctkEnabled", true},
		{"SATA1:2.ctkEnabled", true},
		{"ide0:0.ctkEnabled", true},
		{"nvme0:1.ctkEnabled", true},
		{"ctkEnabled", false},
		{"other0:0.ctkEnabled", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := hasDiskPrefix(tt.key); got != tt.expected {
				t.Errorf("hasDiskPrefix(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestDiskBusAddress(t *testing.T) {
	tests := []struct {
		bus        string
		busNumber  int32
		unitNumber int32
		want       string
	}{
		{"scsi", 0, 0, "scsi0:0"},
		{"scsi", 0, 1, "scsi0:1"},
		{"scsi", 1, 1, "scsi1:1"},
		{"sata", 0, 1, "sata0:1"},
		{"nvme", 0, 2, "nvme0:2"},
		{"ide", 0, 0, "ide0:0"},
		{"scsi", 2, 0, "scsi2:0"},
		{"", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := diskBusAddress(tt.bus, tt.busNumber, tt.unitNumber)
			if got != tt.want {
				t.Errorf("diskBusAddress(%q, %d, %d) = %q, want %q",
					tt.bus, tt.busNumber, tt.unitNumber, got, tt.want)
			}
		})
	}
}

func TestUpdateDisks_setsBusAddress(t *testing.T) {
	unit := int32(1)
	v := &VmAdapter{
		model: model.VM{
			Controllers: []model.Controller{{Key: 1000, BusNumber: 0, Bus: SCSI}},
		},
	}
	v.updateDisks(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{
		&types.VirtualDisk{
			VirtualDevice: types.VirtualDevice{
				Key:           2001,
				ControllerKey: 1000,
				UnitNumber:    &unit,
				Backing: &types.VirtualDiskFlatVer2BackingInfo{
					VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
						FileName: "[ds] vm/disk.vmdk",
					},
				},
			},
			CapacityInBytes: 1024,
		},
	}})
	if len(v.model.Disks) != 1 {
		t.Fatalf("got %d disks, want 1", len(v.model.Disks))
	}
	if v.model.Disks[0].BusAddress != "scsi0:1" {
		t.Errorf("BusAddress = %q, want scsi0:1", v.model.Disks[0].BusAddress)
	}
	if v.model.Disks[0].Bus != SCSI {
		t.Errorf("Bus = %q, want %q", v.model.Disks[0].Bus, SCSI)
	}
}

func TestUpdateDisks_usesBusNumberWhenKeyDiffers(t *testing.T) {
	unit := int32(0)
	v := &VmAdapter{}
	v.updateControllers(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{
		&types.ParaVirtualSCSIController{
			VirtualSCSIController: types.VirtualSCSIController{
				VirtualController: types.VirtualController{
					VirtualDevice: types.VirtualDevice{Key: 1000},
					BusNumber:     2,
					Device:        []int32{2000},
				},
			},
		},
	}})
	if len(v.model.Controllers) != 1 {
		t.Fatalf("got %d controllers, want 1", len(v.model.Controllers))
	}
	if v.model.Controllers[0].Key != 1000 {
		t.Errorf("Key = %d, want 1000", v.model.Controllers[0].Key)
	}
	if v.model.Controllers[0].BusNumber != 2 {
		t.Errorf("BusNumber = %d, want 2", v.model.Controllers[0].BusNumber)
	}

	v.updateDisks(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{
		&types.VirtualDisk{
			VirtualDevice: types.VirtualDevice{
				Key:           2000,
				ControllerKey: 1000,
				UnitNumber:    &unit,
				Backing: &types.VirtualDiskFlatVer2BackingInfo{
					VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
						FileName: "[ds] vm/disk.vmdk",
					},
				},
			},
			CapacityInBytes: 1024,
		},
	}})
	if len(v.model.Disks) != 1 {
		t.Fatalf("got %d disks, want 1", len(v.model.Disks))
	}
	if v.model.Disks[0].BusAddress != "scsi2:0" {
		t.Errorf("BusAddress = %q, want scsi2:0 (from BusNumber, not Key remainder)", v.model.Disks[0].BusAddress)
	}
	if v.model.Disks[0].BusNumber != 2 {
		t.Errorf("Disk.BusNumber = %d, want 2", v.model.Disks[0].BusNumber)
	}
}

func TestVmAdapterApplyVAppProduct(t *testing.T) {
	v := &VmAdapter{}
	v.Apply(types.ObjectUpdate{
		ChangeSet: []types.PropertyChange{
			{
				Op:   Assign,
				Name: fVAppProduct,
				Val: types.ArrayOfVAppProductInfo{
					VAppProductInfo: []types.VAppProductInfo{
						{
							Key:         1,
							ClassId:     "com.example",
							InstanceId:  "appliance",
							Name:        "Example Appliance",
							Vendor:      "Example",
							Version:     "1.2",
							FullVersion: "1.2.3-build4",
							VendorUrl:   "https://example.com",
							ProductUrl:  "https://example.com/product",
							AppUrl:      "https://${vm.ip}/",
						},
					},
				},
			},
		},
	})

	want := []model.VAppProduct{
		{
			Key:         1,
			ClassId:     "com.example",
			InstanceId:  "appliance",
			Name:        "Example Appliance",
			Vendor:      "Example",
			Version:     "1.2",
			FullVersion: "1.2.3-build4",
			VendorUrl:   "https://example.com",
			ProductUrl:  "https://example.com/product",
			AppUrl:      "https://${vm.ip}/",
		},
	}
	if !reflect.DeepEqual(v.model.VAppProducts, want) {
		t.Errorf("VAppProducts mismatch\ngot:  %+v\nwant: %+v", v.model.VAppProducts, want)
	}

	// A VM without a vApp config reports the property unset; stale products are cleared.
	v.Apply(types.ObjectUpdate{
		ChangeSet: []types.PropertyChange{
			{
				Op:   Assign,
				Name: fVAppProduct,
			},
		},
	})
	if v.model.VAppProducts != nil {
		t.Errorf("VAppProducts = %+v, want nil", v.model.VAppProducts)
	}
}

func storageJSON(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func storageFields(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()
	for key, value := range want {
		if !reflect.DeepEqual(got[key], value) {
			t.Errorf("%s: got %#v, want %#v", key, got[key], value)
		}
	}
}

func storageUpdate(name string, value any) types.ObjectUpdate {
	return types.ObjectUpdate{ChangeSet: []types.PropertyChange{{Name: name, Op: types.PropertyChangeOpAssign, Val: value}}}
}

func storageHostJSON(t *testing.T, a *HostAdapter) map[string]any {
	t.Helper()
	var resource web.Host
	resource.With(&a.model)
	return storageJSON(t, &resource)
}

func TestStorageDiskOptionalValues(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name    string
		backing types.BaseVirtualDeviceBackingInfo
		want    map[string]any
	}{
		{"thin", &types.VirtualDiskFlatVer2BackingInfo{ThinProvisioned: &yes, EagerlyScrub: &no}, map[string]any{"thinProvisioned": true, "eagerlyScrub": false}},
		{"thick", &types.VirtualDiskFlatVer2BackingInfo{ThinProvisioned: &no, EagerlyScrub: &yes}, map[string]any{"thinProvisioned": false, "eagerlyScrub": true}},
		{"unknown", &types.VirtualDiskFlatVer2BackingInfo{}, map[string]any{"thinProvisioned": nil, "eagerlyScrub": nil}},
		{"flat v1 unknown", &types.VirtualDiskFlatVer1BackingInfo{}, map[string]any{"thinProvisioned": nil, "eagerlyScrub": nil}},
		{"physical RDM", &types.VirtualDiskRawDiskMappingVer1BackingInfo{CompatibilityMode: "physicalMode", Uuid: "backing-1", LunUuid: "lun-1", DeviceName: "/vmfs/devices/disks/naa.01"}, map[string]any{"compatibilityMode": "physicalMode", "backingUuid": "backing-1", "lunUuid": "lun-1", "physicalMode": true, "thinProvisioned": nil}},
		{"virtual RDM", &types.VirtualDiskRawDiskMappingVer1BackingInfo{CompatibilityMode: "virtualMode"}, map[string]any{"compatibilityMode": "virtualMode", "physicalMode": nil}},
		{"unknown RDM mode", &types.VirtualDiskRawDiskMappingVer1BackingInfo{}, map[string]any{"compatibilityMode": nil, "physicalMode": nil}},
		{"raw v2 UUID is not LUN UUID", &types.VirtualDiskRawDiskVer2BackingInfo{Uuid: "backing-2"}, map[string]any{"compatibilityMode": "physicalMode", "backingUuid": "backing-2", "lunUuid": nil, "physicalMode": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit := int32(0)
			a := VmAdapter{}
			a.updateDisks(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{&types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2000, UnitNumber: &unit, Backing: tt.backing}, CapacityInBytes: 1024}}})
			if len(a.model.Disks) != 1 {
				t.Fatalf("disks: %#v", a.model.Disks)
			}
			storageFields(t, storageJSON(t, a.model.Disks[0]), tt.want)
		})
	}
	var old model.Disk
	if err := json.Unmarshal([]byte(`{"rdm":true}`), &old); err != nil {
		t.Fatal(err)
	}
	storageFields(t, storageJSON(t, old), map[string]any{"compatibilityMode": nil, "thinProvisioned": nil, "eagerlyScrub": nil, "backingUuid": nil})
}

func TestStorageHostAdapterAndLunIdentity(t *testing.T) {
	a := HostAdapter{}
	fc := &types.HostFibreChannelHba{HostHostBusAdapter: types.HostHostBusAdapter{Key: "hba-1", Device: "vmhba1", Pci: "0000:03:00.0", Model: "adapter model", Driver: "driver"}, Speed: 16000000000, NodeWorldWideName: 1, PortWorldWideName: 2}
	a.Apply(storageUpdate(fHostBusAdapter, types.ArrayOfHostHostBusAdapter{HostHostBusAdapter: []types.BaseHostHostBusAdapter{fc, &types.HostTcpHba{HostHostBusAdapter: types.HostHostBusAdapter{Key: "software", Device: "vmhba64"}}}}))
	// PCI inventory may arrive after the HBA, including after reloading SQLite.
	a.model.ID = "host-1"
	db := libmodel.New(filepath.Join(t.TempDir(), "inventory.db"), &model.Host{})
	if err := db.Open(true); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(true) }()
	if err := db.Insert(&a.model); err != nil {
		t.Fatal(err)
	}
	a = HostAdapter{model: model.Host{Base: model.Base{ID: "host-1"}}}
	if err := db.Get(&a.model); err != nil {
		t.Fatal(err)
	}
	a.Apply(storageUpdate("hardware.pciDevice", types.ArrayOfHostPciDevice{HostPciDevice: []types.HostPciDevice{{Id: "0000:03:00.0", VendorName: " Adapter vendor "}, {Id: "0000:04:00.0", VendorName: "wrong vendor"}}}))
	a.Apply(storageUpdate(fScsiLun, types.ArrayOfScsiLun{ScsiLun: []types.BaseScsiLun{&types.HostScsiDisk{ScsiLun: types.ScsiLun{Key: "lun-key", CanonicalName: "naa.0001", Uuid: "lun-uuid", Vendor: " LUN vendor ", Model: " LUN model "}}}}))
	j := storageHostJSON(t, &a)
	hbas := j["hbaDiskInfo"].([]any)
	storageFields(t, hbas[0].(map[string]any), map[string]any{"type": "HostFibreChannelHba", "pci": "0000:03:00.0", "vendor": "Adapter vendor", "speedBitsPerSecond": float64(16000000000), "nodeWorldWideName": "0000000000000001", "portWorldWideName": "0000000000000002"})
	storageFields(t, hbas[1].(map[string]any), map[string]any{"type": "HostTcpHba", "vendor": nil, "speedBitsPerSecond": nil})
	storageFields(t, j["hostScsiDisks"].([]any)[0].(map[string]any), map[string]any{"uuid": "lun-uuid", "canonicalName": "naa.0001", "key": "lun-key", "vendor": "LUN vendor"})
	// A later HBA refresh must preserve its exact PCI correlation.
	a.Apply(storageUpdate(fHostBusAdapter, types.ArrayOfHostHostBusAdapter{HostHostBusAdapter: []types.BaseHostHostBusAdapter{fc}}))
	storageFields(t, storageHostJSON(t, &a)["hbaDiskInfo"].([]any)[0].(map[string]any), map[string]any{"vendor": "Adapter vendor"})
	a.Apply(types.ObjectUpdate{ChangeSet: []types.PropertyChange{{Name: "hardware.pciDevice", Op: types.PropertyChangeOpRemove}}})
	storageFields(t, storageHostJSON(t, &a)["hbaDiskInfo"].([]any)[0].(map[string]any), map[string]any{"vendor": nil})
}

func TestStorageMultipathPreservesPaths(t *testing.T) {
	yes, no := true, false
	a := HostAdapter{}
	a.Apply(storageUpdate("config.storageDevice.multipathInfo", types.HostMultipathInfo{Lun: []types.HostMultipathInfoLogicalUnit{{Key: "mp-1", Id: "naa.01", Lun: "lun-1", Policy: &types.HostMultipathInfoFixedLogicalUnitPolicy{HostMultipathInfoLogicalUnitPolicy: types.HostMultipathInfoLogicalUnitPolicy{Policy: "VMW_PSP_FIXED"}, Prefer: "path-1"}, StorageArrayTypePolicy: &types.HostMultipathInfoLogicalUnitStorageArrayTypePolicy{Policy: "VMW_SATP_ALUA"}, Path: []types.HostMultipathInfoPath{
		{Key: "path-1", Name: "vmhba1:C0:T0:L0", Adapter: "hba-1", Lun: "lun-1", PathState: "active", State: "active", IsWorkingPath: &yes, Transport: &types.HostFibreChannelTargetTransport{NodeWorldWideName: 1, PortWorldWideName: 2}},
		{Key: "path-2", Name: "vmhba1:C0:T1:L0", Adapter: "hba-1", Lun: "lun-1", PathState: "standby", IsWorkingPath: &no, Transport: &types.HostInternetScsiTargetTransport{IScsiName: "iqn.target", Address: []string{"10.0.0.1:3260", "10.0.0.2:3260"}}},
		{Key: "path-3", Adapter: "hba-2", Lun: "lun-1", PathState: "dead"},
	}}}}))
	j := storageHostJSON(t, &a)
	luns, ok := j["hostMultipathInfo"].([]any)
	if !ok || len(luns) != 1 {
		t.Fatalf("multipath missing: %#v", j["hostMultipathInfo"])
	}
	lun := luns[0].(map[string]any)
	storageFields(t, lun, map[string]any{"key": "mp-1", "id": "naa.01", "lun": "lun-1", "policy": "VMW_PSP_FIXED", "preferredPath": "path-1", "storageArrayTypePolicy": "VMW_SATP_ALUA"})
	paths := lun["paths"].([]any)
	if len(paths) != 3 {
		t.Fatalf("lost paths: %#v", paths)
	}
	storageFields(t, paths[0].(map[string]any), map[string]any{"adapter": "hba-1", "lun": "lun-1", "pathState": "active", "state": "active", "isWorkingPath": true})
	storageFields(t, paths[0].(map[string]any)["transport"].(map[string]any), map[string]any{"type": "HostFibreChannelTargetTransport", "protocol": "FibreChannel", "nodeWorldWideName": "0000000000000001"})
	storageFields(t, paths[1].(map[string]any), map[string]any{"isWorkingPath": false, "pathState": "standby"})
	storageFields(t, paths[1].(map[string]any)["transport"].(map[string]any), map[string]any{"iscsiName": "iqn.target", "addresses": []any{"10.0.0.1:3260", "10.0.0.2:3260"}})
	storageFields(t, paths[2].(map[string]any), map[string]any{"isWorkingPath": nil, "pathState": "dead"})
	a.Apply(types.ObjectUpdate{ChangeSet: []types.PropertyChange{{Name: "config.storageDevice.multipathInfo", Op: types.PropertyChangeOpRemove}}})
	if v := storageHostJSON(t, &a)["hostMultipathInfo"]; v != nil {
		t.Fatalf("stale multipath: %#v", v)
	}
}

func TestStorageNvmeTopologyTransports(t *testing.T) {
	for _, transport := range []string{"fibreChannel", "tcp", "rdma", "pcie", "unsupported", "future"} {
		t.Run(transport, func(t *testing.T) {
			a := HostAdapter{}
			a.Apply(storageUpdate("config.storageDevice.nvmeTopology.adapter", types.ArrayOfHostNvmeTopologyInterface{HostNvmeTopologyInterface: []types.HostNvmeTopologyInterface{{Key: "interface-1", Adapter: "hba-1", ConnectedController: []types.HostNvmeController{{Key: "ctrl-1", ControllerNumber: 7, Subnqn: "nqn.2026.test:array", Name: "nvme-controller", AssociatedAdapter: "hba-1", TransportType: transport, VendorId: "0001", Model: "NVMe array", SerialNumber: "serial-1", AttachedNamespace: []types.HostNvmeNamespace{{Key: "ns-1", Name: "eui.0001", Id: 1, BlockSize: 512, CapacityInBlocks: 2048}, {Key: "ns-2", Name: "eui.0002", Id: 2, BlockSize: 4096, CapacityInBlocks: 4096}}}}}}}))
			j := storageHostJSON(t, &a)
			topology, ok := j["hostNvmeTopology"].([]any)
			if !ok || len(topology) != 1 {
				t.Fatalf("NVMe missing: %#v", j["hostNvmeTopology"])
			}
			adapter := topology[0].(map[string]any)
			storageFields(t, adapter, map[string]any{"key": "interface-1", "adapter": "hba-1"})
			controller := adapter["connectedController"].([]any)[0].(map[string]any)
			storageFields(t, controller, map[string]any{"key": "ctrl-1", "controllerNumber": float64(7), "subnqn": "nqn.2026.test:array", "associatedAdapter": "hba-1", "transportType": transport, "vendorId": "0001"})
			ns := controller["attachedNamespace"].([]any)
			if len(ns) != 2 {
				t.Fatalf("lost namespaces: %#v", ns)
			}
			storageFields(t, ns[0].(map[string]any), map[string]any{"id": float64(1), "name": "eui.0001", "blockSize": float64(512), "capacityInBlocks": float64(2048)})
			a.Apply(storageUpdate("config.storageDevice.nvmeTopology.adapter", types.ArrayOfHostNvmeTopologyInterface{HostNvmeTopologyInterface: []types.HostNvmeTopologyInterface{{Key: "interface-1", Adapter: "hba-1"}}}))
			adapter = storageHostJSON(t, &a)["hostNvmeTopology"].([]any)[0].(map[string]any)
			if v := adapter["connectedController"]; v != nil {
				t.Fatalf("stale controllers: %#v", v)
			}
		})
	}
}

func TestStorageVvolArraysStayDistinct(t *testing.T) {
	a := DatastoreAdapter{}
	a.Apply(storageUpdate(fVmfsExtent, &types.VvolDatastoreInfo{VvolDS: &types.HostVvolVolume{ScId: "container-1", StorageArray: []types.VASAStorageArray{{Name: "array one", Uuid: "array-uuid-1", VendorId: "vendor-1", ModelId: "model-1"}, {Name: "array two", Uuid: "array-uuid-2"}}}}))
	var r web.Datastore
	r.With(&a.model)
	j := storageJSON(t, &r)
	arrays, ok := j["storageArrays"].([]any)
	if !ok || len(arrays) != 2 {
		t.Fatalf("arrays missing: %#v", j["storageArrays"])
	}
	storageFields(t, arrays[0].(map[string]any), map[string]any{"uuid": "array-uuid-1", "name": "array one", "vendorId": "vendor-1", "modelId": "model-1"})
	storageFields(t, j, map[string]any{"storageContainerId": "container-1"})
	a.Apply(storageUpdate(fVmfsExtent, types.VvolDatastoreInfo{}))
	r.With(&a.model)
	storageFields(t, storageJSON(t, &r), map[string]any{"storageArrays": nil, "storageContainerId": nil})
}

func TestStorageVvolMetadataRemoval(t *testing.T) {
	for _, change := range []types.PropertyChange{
		{Name: fVmfsExtent, Op: types.PropertyChangeOpAssign, Val: nil},
		{Name: fVmfsExtent, Op: types.PropertyChangeOpRemove},
		{Name: fVmfsExtent, Op: types.PropertyChangeOpAssign, Val: types.VsanDatastoreInfo{}},
		{Name: fVmfsExtent, Op: types.PropertyChangeOpAssign, Val: (*types.VvolDatastoreInfo)(nil)},
	} {
		t.Run(string(change.Op)+"/"+storageTypeName(change.Val), func(t *testing.T) {
			a := DatastoreAdapter{}
			a.Apply(storageUpdate(fVmfsExtent, types.VvolDatastoreInfo{VvolDS: &types.HostVvolVolume{ScId: "container-1", StorageArray: []types.VASAStorageArray{{Uuid: "array-1"}}}}))
			a.Apply(types.ObjectUpdate{ChangeSet: []types.PropertyChange{change}})
			var resource web.Datastore
			resource.With(&a.model)
			storageFields(t, storageJSON(t, &resource), map[string]any{"storageArrays": nil, "storageContainerId": nil})
		})
	}
}

func TestStorageHostPartialAndEmptyUpdates(t *testing.T) {
	a := HostAdapter{}
	a.Apply(storageUpdate(fHostBusAdapter, types.ArrayOfHostHostBusAdapter{HostHostBusAdapter: []types.BaseHostHostBusAdapter{nil, (*types.HostFibreChannelHba)(nil), &types.HostFibreChannelOverEthernetHba{HostFibreChannelHba: types.HostFibreChannelHba{HostHostBusAdapter: types.HostHostBusAdapter{Pci: "pci-1"}, Speed: 0, NodeWorldWideName: -1}}}}))
	a.Apply(storageUpdate(fPciDevice, types.ArrayOfHostPciDevice{HostPciDevice: []types.HostPciDevice{{Id: "pci-1", VendorName: "one"}, {Id: "pci-1", VendorName: "two"}}}))
	a.Apply(storageUpdate(fScsiLun, types.ArrayOfScsiLun{ScsiLun: []types.BaseScsiLun{nil, (*types.HostScsiDisk)(nil)}}))
	hbas := storageHostJSON(t, &a)["hbaDiskInfo"].([]any)
	if len(hbas) != 1 {
		t.Fatalf("HBA count: %d", len(hbas))
	}
	storageFields(t, hbas[0].(map[string]any), map[string]any{"type": "HostFibreChannelOverEthernetHba", "protocol": "FCoE", "speedBitsPerSecond": float64(0), "nodeWorldWideName": "ffffffffffffffff", "vendor": nil})
	a.Apply(storageUpdate(fMultipathInfo, &types.HostMultipathInfo{Lun: []types.HostMultipathInfoLogicalUnit{{Lun: "one", Policy: (*types.HostMultipathInfoFixedLogicalUnitPolicy)(nil), Path: []types.HostMultipathInfoPath{{Transport: (*types.HostFibreChannelTargetTransport)(nil)}}}, {Lun: "two"}}}))
	if len(a.model.HostMultipathInfo) != 2 {
		t.Fatal("lost orphan LUNs")
	}
	a.Apply(storageUpdate(fMultipathInfo, (*types.HostMultipathInfo)(nil)))
	a.Apply(storageUpdate(fNvmeTopology, types.ArrayOfHostNvmeTopologyInterface{}))
	a.Apply(storageUpdate(fHostBusAdapter, types.ArrayOfHostHostBusAdapter{}))
	storageFields(t, storageHostJSON(t, &a), map[string]any{"hostMultipathInfo": nil, "hostNvmeTopology": nil, "hbaDiskInfo": nil, "hostScsiDisks": nil})
}

func TestStorageSQLiteToREST(t *testing.T) {
	no := false
	unit := int32(0)
	host := HostAdapter{model: model.Host{Base: model.Base{ID: "host-1"}}}
	host.Apply(storageUpdate(fScsiLun, types.ArrayOfScsiLun{ScsiLun: []types.BaseScsiLun{&types.HostScsiDisk{ScsiLun: types.ScsiLun{Uuid: "host-lun-1", CanonicalName: "naa.01", Key: "lun-key", Vendor: " LUN vendor ", Model: " LUN model "}}}}))
	host.Apply(storageUpdate(fHostBusAdapter, types.ArrayOfHostHostBusAdapter{HostHostBusAdapter: []types.BaseHostHostBusAdapter{&types.HostFibreChannelHba{HostHostBusAdapter: types.HostHostBusAdapter{Pci: "pci-1", Key: "hba-1", Device: "vmhba1", Driver: "lpfc", Model: "FC adapter"}, Speed: 16000000000, NodeWorldWideName: 1, PortWorldWideName: 2}}}))
	// #8960 must continue to retain every LUN, including multiple LUNs per target.
	topology := storageUpdate(fScsiTopology, types.ArrayOfHostScsiTopologyInterface{HostScsiTopologyInterface: []types.HostScsiTopologyInterface{{Adapter: "hba-1", Target: []types.HostScsiTopologyTarget{{Lun: []types.HostScsiTopologyLun{{ScsiLun: "lun-key"}, {ScsiLun: "lun-key-2"}, {}}}, {}}}}})
	host.Apply(topology)
	host.Apply(topology) // Refresh replaces the topology rather than appending it.
	host.Apply(storageUpdate(fPciDevice, types.ArrayOfHostPciDevice{HostPciDevice: []types.HostPciDevice{{Id: "pci-1", VendorName: "adapter vendor"}}}))
	host.Apply(storageUpdate(fMultipathInfo, types.HostMultipathInfo{Lun: []types.HostMultipathInfoLogicalUnit{{Lun: "lun-key", Path: []types.HostMultipathInfoPath{{Key: "path-1", Adapter: "hba-1", IsWorkingPath: &no, Transport: &types.HostFibreChannelOverEthernetTargetTransport{HostFibreChannelTargetTransport: types.HostFibreChannelTargetTransport{NodeWorldWideName: 1, PortWorldWideName: 2}, VlanId: 0, VnportMac: "00:01:02:03:04:05"}}}}}}))
	host.Apply(storageUpdate(fNvmeTopology, types.ArrayOfHostNvmeTopologyInterface{HostNvmeTopologyInterface: []types.HostNvmeTopologyInterface{{Adapter: "hba-1", ConnectedController: []types.HostNvmeController{{Key: "controller-1", TransportType: "tcp", AttachedNamespace: []types.HostNvmeNamespace{{Id: 9, Name: "ns-9"}}}}}}}))
	ds := DatastoreAdapter{model: model.Datastore{Base: model.Base{ID: "ds-1"}}}
	ds.Apply(storageUpdate(fVmfsExtent, types.VvolDatastoreInfo{VvolDS: &types.HostVvolVolume{HostFileSystemVolume: types.HostFileSystemVolume{Type: "VVOL"}, ProtocolEndpointType: "SCSI", HostPE: []types.VVolHostPE{{ProtocolEndpoint: []types.HostProtocolEndpoint{{DeviceId: " naa.pe-1 "}, {DeviceId: "naa.pe-1"}, {DeviceId: ""}, {DeviceId: "naa.pe-2"}}}}, ScId: "sc-1", StorageArray: []types.VASAStorageArray{{Uuid: "array-1", Name: "array", VendorId: "vendor", ModelId: "model"}}}}))
	vmfs := DatastoreAdapter{model: model.Datastore{Base: model.Base{ID: "ds-vmfs"}}}
	vmfs.Apply(storageUpdate(fVmfsExtent, types.VmfsDatastoreInfo{Vmfs: &types.HostVmfsVolume{HostFileSystemVolume: types.HostFileSystemVolume{Type: "VMFS"}, MajorVersion: 6, Extent: []types.HostScsiDiskPartition{{DiskName: "naa.01"}, {DiskName: "naa.02"}}}}))
	nfs := DatastoreAdapter{model: model.Datastore{Base: model.Base{ID: "ds-nfs"}}}
	nfs.Apply(storageUpdate(fVmfsExtent, types.NasDatastoreInfo{Nas: &types.HostNasVolume{HostFileSystemVolume: types.HostFileSystemVolume{Type: "NFS41"}, RemoteHost: "10.0.0.1", RemotePath: "/export", RemoteHostNames: []string{"10.0.0.1", "10.0.0.2"}}}))
	vm := VmAdapter{model: model.VM{Base: model.Base{ID: "vm-1"}, Host: "host-1"}}
	vm.updateDisks(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{
		&types.VirtualDisk{VirtualDevice: types.VirtualDevice{UnitNumber: &unit, Backing: &types.VirtualDiskFlatVer2BackingInfo{ThinProvisioned: &no, EagerlyScrub: &no}}},
		&types.VirtualDisk{VirtualDevice: types.VirtualDevice{UnitNumber: &unit, Backing: &types.VirtualDiskRawDiskMappingVer1BackingInfo{LunUuid: "host-lun-1", Uuid: "backing-1", CompatibilityMode: "virtualMode"}}},
	}})
	db := libmodel.New(filepath.Join(t.TempDir(), "storage.db"), &model.Host{}, &model.Datastore{}, &model.VM{})
	if err := db.Open(true); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(true) }()
	for _, m := range []libmodel.Model{&host.model, &ds.model, &vmfs.model, &nfs.model, &vm.model} {
		if err := db.Insert(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(false); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(false); err != nil {
		t.Fatal(err)
	}
	server := storageAPIServer(t, db)
	storageCheckAPI(t, server, "hosts", "host-1", func(t *testing.T, hj map[string]any) {
		storageFields(t, hj["hbaDiskInfo"].([]any)[0].(map[string]any), map[string]any{"vendor": "adapter vendor", "type": "HostFibreChannelHba", "pci": "pci-1", "speedBitsPerSecond": float64(16000000000), "hbaDevice": "vmhba1", "key": "hba-1", "model": "FC adapter", "driver": "lpfc", "protocol": "FibreChannel", "nodeWorldWideName": "0000000000000001", "portWorldWideName": "0000000000000002"})
		storageFields(t, hj["hostScsiDisks"].([]any)[0].(map[string]any), map[string]any{"uuid": "host-lun-1", "canonicalName": "naa.01", "key": "lun-key", "vendor": "LUN vendor", "model": "LUN model"})
		storageFields(t, hj, map[string]any{"hostScsiTopology": []any{map[string]any{"key": "hba-1", "scsiDiskKeys": []any{"lun-key", "lun-key-2"}}}})
		path := hj["hostMultipathInfo"].([]any)[0].(map[string]any)["paths"].([]any)[0].(map[string]any)
		storageFields(t, path, map[string]any{"isWorkingPath": false, "adapter": "hba-1"})
		storageFields(t, path["transport"].(map[string]any), map[string]any{"protocol": "FCoE", "vlanId": float64(0), "nodeWorldWideName": "0000000000000001", "vnportMac": "00:01:02:03:04:05"})
		c := hj["hostNvmeTopology"].([]any)[0].(map[string]any)["connectedController"].([]any)[0].(map[string]any)
		storageFields(t, c, map[string]any{"transportType": "tcp", "key": "controller-1"})
		storageFields(t, c["attachedNamespace"].([]any)[0].(map[string]any), map[string]any{"id": float64(9), "name": "ns-9"})
	})
	storageCheckAPI(t, server, "datastores", "ds-1", func(t *testing.T, dj map[string]any) {
		storageFields(t, dj, map[string]any{"storageContainerId": "sc-1", "filesystemType": "VVOL", "protocolEndpointType": "SCSI", "backingDevicesNames": []any{"naa.pe-1", "naa.pe-2"}})
		storageFields(t, dj["storageArrays"].([]any)[0].(map[string]any), map[string]any{"uuid": "array-1", "name": "array", "vendorId": "vendor", "modelId": "model"})
	})
	storageCheckAPI(t, server, "datastores", "ds-vmfs", func(t *testing.T, dj map[string]any) {
		storageFields(t, dj, map[string]any{"filesystemType": "VMFS", "filesystemMajorVersion": float64(6), "backingDevicesNames": []any{"naa.01", "naa.02"}, "storageArrays": nil, "storageContainerId": nil})
	})
	storageCheckAPI(t, server, "datastores", "ds-nfs", func(t *testing.T, dj map[string]any) {
		storageFields(t, dj, map[string]any{"filesystemType": "NFS41", "nasRemoteHost": "10.0.0.1", "nasRemotePath": "/export", "nasRemoteHostNames": []any{"10.0.0.1", "10.0.0.2"}, "storageArrays": nil, "storageContainerId": nil})
	})
	storageCheckAPI(t, server, "vms", "vm-1", func(t *testing.T, vj map[string]any) {
		storageFields(t, vj["disks"].([]any)[0].(map[string]any), map[string]any{"thinProvisioned": false, "eagerlyScrub": false})
		storageFields(t, vj["disks"].([]any)[1].(map[string]any), map[string]any{"compatibilityMode": "virtualMode", "backingUuid": "backing-1", "lunUuid": "host-lun-1"})
	})
	// Reused source identifiers on another host must not acquire host-1's data.
	other := model.Host{Base: model.Base{ID: "host-2"}}
	if err := db.Insert(&other); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&other); err != nil {
		t.Fatal(err)
	}
	var hr web.Host
	hr.With(&host.model)
	hr.With(&other)
	storageFields(t, storageJSON(t, &hr), map[string]any{"hostMultipathInfo": nil, "hostNvmeTopology": nil, "hbaDiskInfo": nil})
}

// Serve a populated inventory without starting a live vCenter connection.
// The adapters, SQLite database, routes and JSON serialization are real.
// Unused collector methods deliberately remain unimplemented.
type storageAPICollector struct {
	libcontainer.Collector
	db libmodel.DB
}

func (c *storageAPICollector) Owner() meta.Object {
	return &api.Provider{ObjectMeta: meta.ObjectMeta{UID: "storage-provider"}}
}
func (c *storageAPICollector) Start() error    { return nil }
func (c *storageAPICollector) HasParity() bool { return true }
func (c *storageAPICollector) DB() libmodel.DB { return c.db }

func storageAPIServer(t *testing.T, db libmodel.DB) *gin.Engine {
	t.Helper()
	authRequired := webbase.Settings.AuthRequired
	webbase.Settings.AuthRequired = false
	t.Cleanup(func() { webbase.Settings.AuthRequired = authRequired })
	container := libcontainer.New()
	if err := container.Add(&storageAPICollector{db: db}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler := web.Handler{Handler: webbase.Handler{Container: container}}
	(&web.HostHandler{Handler: handler}).AddRoutes(router)
	(&web.DatastoreHandler{Handler: handler}).AddRoutes(router)
	(&web.VMHandler{Handler: handler}).AddRoutes(router)
	return router
}

func storageCheckAPI(t *testing.T, server http.Handler, collection, id string, check func(*testing.T, map[string]any)) {
	t.Helper()
	root := "/providers/vsphere/storage-provider/" + collection
	for _, suffix := range []string{"/" + id, "?detail=1", "?detail=all", ""} {
		t.Run(collection+"/"+id+suffix, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, root+suffix, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s: %d %s", root+suffix, response.Code, response.Body.String())
			}
			var resource map[string]any
			if suffix == "/"+id {
				if err := json.Unmarshal(response.Body.Bytes(), &resource); err != nil {
					t.Fatal(err)
				}
			} else {
				var resources []map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &resources); err != nil {
					t.Fatal(err)
				}
				for _, item := range resources {
					if item["id"] == id {
						resource = item
						break
					}
				}
			}
			if resource == nil || resource["id"] != id {
				t.Fatalf("resource %s missing: %s", id, response.Body.String())
			}
			if suffix == "" && collection != "vms" {
				// Host/datastore lists default to summaries; callers must request detail.
				storageFields(t, resource, map[string]any{"hbaDiskInfo": nil, "hostScsiDisks": nil, "hostScsiTopology": nil, "hostMultipathInfo": nil, "hostNvmeTopology": nil, "filesystemType": nil, "storageArrays": nil, "storageContainerId": nil})
				return
			}
			check(t, resource)
		})
	}
}
