package vsphere

import (
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
)

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
