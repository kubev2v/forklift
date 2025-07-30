package vsphere

import (
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
)

// Test getDiskGuestInfo method
func TestVmAdapter_getDiskGuestInfo(t *testing.T) {
	tests := []struct {
		name        string
		guestDisks  []model.GuestDisk
		deviceKey   int32
		expected    *model.GuestDisk
		expectFound bool
	}{
		{
			name: "returns pointer to correct slice element when key exists",
			guestDisks: []model.GuestDisk{
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
			expected: &model.GuestDisk{
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
			guestDisks: []model.GuestDisk{
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
			expected: &model.GuestDisk{
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
			guestDisks: []model.GuestDisk{
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
			expected: &model.GuestDisk{
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
			guestDisks: []model.GuestDisk{
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
			guestDisks:  []model.GuestDisk{},
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
			guestDisks: []model.GuestDisk{
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
			guestDisks: []model.GuestDisk{
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
			expected: &model.GuestDisk{
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

// Test updateOrAppendGuestDisk method
func TestVmAdapter_updateOrAppendGuestDisk(t *testing.T) {
	tests := []struct {
		name               string
		initialGuestDisks  []model.GuestDisk
		initialDisks       []model.Disk
		newDisk            model.GuestDisk
		expectedGuestDisks []model.GuestDisk
		expectedDisks      []model.Disk
		expectedOperation  string // "replace", "append", or "append_and_update_disk"
	}{
		{
			name: "replaces existing GuestDisk when key matches",
			initialGuestDisks: []model.GuestDisk{
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
			initialDisks: []model.Disk{
				{Key: 300, WinDriveLetter: "e"},
				{Key: 400, WinDriveLetter: "f"},
			},
			newDisk: model.GuestDisk{
				Key:            100,
				DiskPath:       "C:\\",
				Capacity:       1500000000, // Updated capacity
				FreeSpace:      800000000,  // Updated free space
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1500000000,
					FreeSpace:      800000000,
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
			expectedDisks: []model.Disk{
				{Key: 300, WinDriveLetter: "e"},
				{Key: 400, WinDriveLetter: "f"},
			},
			expectedOperation: "replace",
		},
		{
			name: "appends new GuestDisk when no existing key is found",
			initialGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
			},
			initialDisks: []model.Disk{
				{Key: 300, WinDriveLetter: "e"},
			},
			newDisk: model.GuestDisk{
				Key:            200,
				DiskPath:       "D:\\",
				Capacity:       2000000000,
				FreeSpace:      1500000000,
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
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
			expectedDisks: []model.Disk{
				{Key: 300, WinDriveLetter: "e"},
			},
			expectedOperation: "append",
		},
		{
			name:              "appends to empty GuestDisk slice",
			initialGuestDisks: []model.GuestDisk{},
			initialDisks: []model.Disk{
				{Key: 300, WinDriveLetter: "e"},
			},
			newDisk: model.GuestDisk{
				Key:            100,
				DiskPath:       "C:\\",
				Capacity:       1000000000,
				FreeSpace:      500000000,
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
			},
			expectedDisks: []model.Disk{
				{Key: 300, WinDriveLetter: "e"},
			},
			expectedOperation: "append",
		},
		{
			name: "propagates Windows drive letter to matching Disk when key exists",
			initialGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "E:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
			},
			initialDisks: []model.Disk{
				{Key: 100, WinDriveLetter: ""},  // Empty drive letter initially
				{Key: 200, WinDriveLetter: "f"}, // Different disk
			},
			newDisk: model.GuestDisk{
				Key:            300,
				DiskPath:       "C:\\",
				Capacity:       2000000000,
				FreeSpace:      1500000000,
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "E:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
				{
					Key:            300,
					DiskPath:       "C:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
			},
			expectedDisks: []model.Disk{
				{Key: 100, WinDriveLetter: ""},  // Unchanged since new disk has different key
				{Key: 200, WinDriveLetter: "f"}, // Unchanged
			},
			expectedOperation: "append_and_update_disk",
		},
		{
			name:              "propagates Windows drive letter update to matching Disk",
			initialGuestDisks: []model.GuestDisk{},
			initialDisks: []model.Disk{
				{Key: 100, WinDriveLetter: ""},  // Empty drive letter initially
				{Key: 200, WinDriveLetter: "f"}, // Different disk
			},
			newDisk: model.GuestDisk{
				Key:            100,
				DiskPath:       "C:\\",
				Capacity:       2000000000,
				FreeSpace:      1500000000,
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "NTFS",
				},
			},
			expectedDisks: []model.Disk{
				{Key: 100, WinDriveLetter: "c"}, // Updated with extracted drive letter
				{Key: 200, WinDriveLetter: "f"}, // Unchanged
			},
			expectedOperation: "append_and_update_disk",
		},
		{
			name:              "handles non-Windows path without updating drive letter",
			initialGuestDisks: []model.GuestDisk{},
			initialDisks: []model.Disk{
				{Key: 100, WinDriveLetter: ""},
			},
			newDisk: model.GuestDisk{
				Key:            100,
				DiskPath:       "/home/user",
				Capacity:       2000000000,
				FreeSpace:      1500000000,
				FilesystemType: "ext4",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "/home/user",
					Capacity:       2000000000,
					FreeSpace:      1500000000,
					FilesystemType: "ext4",
				},
			},
			expectedDisks: []model.Disk{
				{Key: 100, WinDriveLetter: ""}, // Unchanged since it's not a Windows path
			},
			expectedOperation: "append_and_update_disk",
		},
		{
			name: "replaces existing and updates matching Disk drive letter",
			initialGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "E:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
			},
			initialDisks: []model.Disk{
				{Key: 100, WinDriveLetter: "e"}, // Will be updated
				{Key: 200, WinDriveLetter: "f"}, // Different disk, unchanged
			},
			newDisk: model.GuestDisk{
				Key:            100,
				DiskPath:       "C:\\", // Different drive letter
				Capacity:       1500000000,
				FreeSpace:      800000000,
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            100,
					DiskPath:       "C:\\",
					Capacity:       1500000000,
					FreeSpace:      800000000,
					FilesystemType: "NTFS",
				},
			},
			expectedDisks: []model.Disk{
				{Key: 100, WinDriveLetter: "c"}, // Updated with new drive letter
				{Key: 200, WinDriveLetter: "f"}, // Unchanged
			},
			expectedOperation: "replace",
		},
		{
			name: "handles edge case with zero key",
			initialGuestDisks: []model.GuestDisk{
				{
					Key:            0,
					DiskPath:       "C:\\",
					Capacity:       1000000000,
					FreeSpace:      500000000,
					FilesystemType: "NTFS",
				},
			},
			initialDisks: []model.Disk{
				{Key: 0, WinDriveLetter: "c"},
			},
			newDisk: model.GuestDisk{
				Key:            0,
				DiskPath:       "D:\\", // Updated path
				Capacity:       1500000000,
				FreeSpace:      800000000,
				FilesystemType: "NTFS",
			},
			expectedGuestDisks: []model.GuestDisk{
				{
					Key:            0,
					DiskPath:       "D:\\",
					Capacity:       1500000000,
					FreeSpace:      800000000,
					FilesystemType: "NTFS",
				},
			},
			expectedDisks: []model.Disk{
				{Key: 0, WinDriveLetter: "d"}, // Updated with new drive letter
			},
			expectedOperation: "replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup VmAdapter with test data (make copies to avoid test interference)
			v := &VmAdapter{
				model: model.VM{
					GuestDisks: make([]model.GuestDisk, len(tt.initialGuestDisks)),
					Disks:      make([]model.Disk, len(tt.initialDisks)),
				},
			}
			copy(v.model.GuestDisks, tt.initialGuestDisks)
			copy(v.model.Disks, tt.initialDisks)

			// Call the method under test
			v.updateOrAppendGuestDisk(tt.newDisk)

			// Verify GuestDisks slice
			if len(v.model.GuestDisks) != len(tt.expectedGuestDisks) {
				t.Errorf("expected %d guest disks, got %d", len(tt.expectedGuestDisks), len(v.model.GuestDisks))
			}

			for i, expected := range tt.expectedGuestDisks {
				if i >= len(v.model.GuestDisks) {
					t.Errorf("missing expected guest disk at index %d: %+v", i, expected)
					continue
				}
				actual := v.model.GuestDisks[i]
				if actual.Key != expected.Key ||
					actual.DiskPath != expected.DiskPath ||
					actual.Capacity != expected.Capacity ||
					actual.FreeSpace != expected.FreeSpace ||
					actual.FilesystemType != expected.FilesystemType {
					t.Errorf("guest disk at index %d doesn't match.\nExpected: %+v\nGot: %+v", i, expected, actual)
				}
			}

			// Verify Disks slice (drive letter propagation)
			if len(v.model.Disks) != len(tt.expectedDisks) {
				t.Errorf("expected %d disks, got %d", len(tt.expectedDisks), len(v.model.Disks))
			}

			for i, expected := range tt.expectedDisks {
				if i >= len(v.model.Disks) {
					t.Errorf("missing expected disk at index %d: %+v", i, expected)
					continue
				}
				actual := v.model.Disks[i]
				if actual.Key != expected.Key || actual.WinDriveLetter != expected.WinDriveLetter {
					t.Errorf("disk at index %d doesn't match (Key: %d->%d, WinDriveLetter: %s->%s)",
						i, expected.Key, actual.Key, expected.WinDriveLetter, actual.WinDriveLetter)
				}
			}
		})
	}
}
