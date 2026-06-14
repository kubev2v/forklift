package vsphere

import (
	"reflect"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/vmware/govmomi/vim25/types"
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

func TestVmAdapter_Apply_CustomFields(t *testing.T) {
	tests := []struct {
		name              string
		preExistingDef    []model.CustomFieldDef
		preExistingValues []model.CustomFieldValue
		availableFieldVal interface{}
		customValueVal    interface{}
		expectedDef       []model.CustomFieldDef
		expectedValues    []model.CustomFieldValue
	}{
		{
			name: "populates CustomDef and CustomValues from scratch",
			availableFieldVal: types.ArrayOfCustomFieldDef{
				CustomFieldDef: []types.CustomFieldDef{
					{Name: "owner", Key: 100, ManagedObjectType: "VirtualMachine"},
					{Name: "env", Key: 200, ManagedObjectType: ""},
				},
			},
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
			expectedDef: []model.CustomFieldDef{
				{Name: "owner", Key: 100, ManagedObjectType: "VirtualMachine"},
				{Name: "env", Key: 200, ManagedObjectType: ""},
			},
			expectedValues: []model.CustomFieldValue{
				{Key: 100, Value: "alice"},
				{Key: 200, Value: "production"},
			},
		},
		{
			name: "clears CustomDef when vSphere reports empty availableField",
			preExistingDef: []model.CustomFieldDef{
				{Name: "stale", Key: 999},
			},
			availableFieldVal: types.ArrayOfCustomFieldDef{
				CustomFieldDef: []types.CustomFieldDef{},
			},
			expectedDef: []model.CustomFieldDef{},
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
					CustomDef:    tt.preExistingDef,
					CustomValues: tt.preExistingValues,
				},
			}

			changeSet := []types.PropertyChange{}
			if tt.availableFieldVal != nil {
				changeSet = append(changeSet, types.PropertyChange{
					Op:   Assign,
					Name: fAvailableField,
					Val:  tt.availableFieldVal,
				})
			}
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

			if tt.expectedDef != nil {
				if !reflect.DeepEqual(v.model.CustomDef, tt.expectedDef) {
					t.Errorf("CustomDef mismatch\ngot:  %+v\nwant: %+v", v.model.CustomDef, tt.expectedDef)
				}
			}
			if tt.expectedValues != nil {
				if !reflect.DeepEqual(v.model.CustomValues, tt.expectedValues) {
					t.Errorf("CustomValues mismatch\ngot:  %+v\nwant: %+v", v.model.CustomValues, tt.expectedValues)
				}
			}
		})
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
