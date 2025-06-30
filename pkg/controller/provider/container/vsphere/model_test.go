package vsphere

import (
	"strings"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
)

func TestExtractWinDriveLetter(t *testing.T) {
	tests := []struct {
		name     string
		diskPath string
		expected string
	}{
		{
			name:     "Windows C drive",
			diskPath: "C:\\",
			expected: "c",
		},
		{
			name:     "Windows D drive",
			diskPath: "D:\\",
			expected: "d",
		},
		{
			name:     "Windows drive with folder path",
			diskPath: "E:\\Users\\test",
			expected: "e",
		},
		{
			name:     "Unix-style path",
			diskPath: "/home/user",
			expected: "",
		},
		{
			name:     "Empty path",
			diskPath: "",
			expected: "",
		},
		{
			name:     "Invalid Windows path (missing backslash)",
			diskPath: "C:",
			expected: "",
		},
		{
			name:     "Invalid Windows path (wrong format)",
			diskPath: "C:/Users",
			expected: "",
		},
		{
			name:     "Lowercase Windows drive",
			diskPath: "f:\\temp",
			expected: "f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &VmAdapter{}
			result := v.extractWinDriveLetter(tt.diskPath)
			if result != tt.expected {
				t.Errorf("extractWinDriveLetter(%q) = %q, want %q", tt.diskPath, result, tt.expected)
			}
		})
	}
}

func TestUpdateWinDriveLetters(t *testing.T) {
	tests := []struct {
		name        string
		disks       []model.Disk
		guestDisks  []model.GuestDisk
		expected    []string // Expected WinDriveLetter for each disk
		description string
	}{
		{
			name: "Perfect capacity match",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
				{Capacity: 50 * 1024 * 1024 * 1024},  // 50GB
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 95 * 1024 * 1024 * 1024}, // 95GB (5% overhead)
				{DiskPath: "D:\\", Capacity: 48 * 1024 * 1024 * 1024}, // 48GB (4% overhead)
			},
			expected:    []string{"c", "d"},
			description: "Disks should match by capacity when within tolerance",
		},
		{
			name: "Capacity match with maximum tolerance (15%)",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 85 * 1024 * 1024 * 1024}, // 85GB (15% overhead)
			},
			expected:    []string{"c"},
			description: "Should match at the edge of tolerance (15%)",
		},
		{
			name: "No capacity match - falls back to index",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
				{Capacity: 50 * 1024 * 1024 * 1024},  // 50GB
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 80 * 1024 * 1024 * 1024}, // 80GB (20% overhead - exceeds tolerance)
				{DiskPath: "D:\\", Capacity: 25 * 1024 * 1024 * 1024}, // 25GB (50% overhead - exceeds tolerance)
			},
			expected:    []string{"c", "d"},
			description: "Should fall back to index-based matching when capacity is out of tolerance",
		},
		{
			name: "Mixed Windows and Unix paths",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024},
				{Capacity: 50 * 1024 * 1024 * 1024},
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 95 * 1024 * 1024 * 1024},
				{DiskPath: "/home", Capacity: 48 * 1024 * 1024 * 1024},
			},
			expected:    []string{"c", ""},
			description: "Should extract Windows drive letters only, empty for Unix paths",
		},
		{
			name: "More disks than guest disks",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024},
				{Capacity: 50 * 1024 * 1024 * 1024},
				{Capacity: 25 * 1024 * 1024 * 1024},
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 95 * 1024 * 1024 * 1024},
				{DiskPath: "D:\\", Capacity: 48 * 1024 * 1024 * 1024},
			},
			expected:    []string{"c", "d", ""},
			description: "Third disk should have empty drive letter when no guest disk available",
		},
		{
			name: "Capacity matching with best fit selection",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 90 * 1024 * 1024 * 1024}, // 90GB (10% overhead)
				{DiskPath: "D:\\", Capacity: 85 * 1024 * 1024 * 1024}, // 85GB (15% overhead)
			},
			expected:    []string{"c"},
			description: "Should choose the guest disk with smaller capacity difference (better fit)",
		},
		{
			name: "Prevent duplicate matching",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
				{Capacity: 99 * 1024 * 1024 * 1024},  // 99GB
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 95 * 1024 * 1024 * 1024}, // 95GB
				{DiskPath: "D:\\", Capacity: 48 * 1024 * 1024 * 1024}, // 48GB
			},
			expected:    []string{"c", "d"},
			description: "Second disk should fall back to index matching since first guest disk is already matched",
		},
		{
			name:        "Empty disks and guest disks",
			disks:       []model.Disk{},
			guestDisks:  []model.GuestDisk{},
			expected:    []string{},
			description: "Should handle empty input gracefully",
		},
		{
			name: "Guest capacity larger than disk capacity (should not match)",
			disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
			},
			guestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 110 * 1024 * 1024 * 1024}, // 110GB (guest larger than disk)
				{DiskPath: "D:\\", Capacity: 95 * 1024 * 1024 * 1024},  // 95GB
			},
			expected:    []string{"d"},
			description: "Should find best matching guest disk even if one is larger than the disk capacity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a VmAdapter with the test data
			vm := model.VM{
				Disks:      tt.disks,
				GuestDisks: tt.guestDisks,
			}

			adapter := &VmAdapter{
				model: vm,
			}

			// Call the function under test
			adapter.updateWinDriveLetters()

			// Verify the results
			if len(adapter.model.Disks) != len(tt.expected) {
				t.Errorf("Number of disks %d should match expected %d", len(adapter.model.Disks), len(tt.expected))
				return
			}

			for i, expectedLetter := range tt.expected {
				if i < len(adapter.model.Disks) {
					actual := adapter.model.Disks[i].WinDriveLetter
					if actual != expectedLetter {
						t.Errorf("Disk %d should have drive letter '%s', got '%s'", i, expectedLetter, actual)
					}
				}
			}
		})
	}
}

func TestUpdateWinDriveLetters_ComplexScenarios(t *testing.T) {
	t.Run("Multiple exact capacity matches - should use best fit", func(t *testing.T) {
		vm := model.VM{
			Disks: []model.Disk{
				{Capacity: 100 * 1024 * 1024 * 1024}, // 100GB
				{Capacity: 200 * 1024 * 1024 * 1024}, // 200GB
			},
			GuestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 90 * 1024 * 1024 * 1024},  // 90GB (10% overhead from 100GB)
				{DiskPath: "D:\\", Capacity: 180 * 1024 * 1024 * 1024}, // 180GB (10% overhead from 200GB)
				{DiskPath: "E:\\", Capacity: 170 * 1024 * 1024 * 1024}, // 170GB (15% overhead from 200GB)
			},
		}

		adapter := &VmAdapter{model: vm}
		adapter.updateWinDriveLetters()

		// First disk (100GB) should match with first guest disk (90GB)
		if adapter.model.Disks[0].WinDriveLetter != "c" {
			t.Errorf("First disk should have drive letter 'c', got '%s'", adapter.model.Disks[0].WinDriveLetter)
		}
		// Second disk (200GB) should match with second guest disk (180GB) as it's a better fit than third (170GB)
		if adapter.model.Disks[1].WinDriveLetter != "d" {
			t.Errorf("Second disk should have drive letter 'd', got '%s'", adapter.model.Disks[1].WinDriveLetter)
		}
	})

	t.Run("Real-world scenario with different filesystem overhead", func(t *testing.T) {
		vm := model.VM{
			Disks: []model.Disk{
				{Capacity: 536870912000}, // 500GB raw disk
				{Capacity: 107374182400}, // 100GB raw disk
				{Capacity: 21474836480},  // 20GB raw disk
			},
			GuestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: 510000000000}, // C drive with ~5% filesystem overhead
				{DiskPath: "D:\\", Capacity: 95000000000},  // D drive with ~5% filesystem overhead
				{DiskPath: "E:\\", Capacity: 18000000000},  // E drive with ~10% filesystem overhead
			},
		}

		adapter := &VmAdapter{model: vm}
		adapter.updateWinDriveLetters()

		expected := []string{"c", "d", "e"}
		for i, expectedLetter := range expected {
			actual := adapter.model.Disks[i].WinDriveLetter
			if actual != expectedLetter {
				t.Errorf("Disk %d should have drive letter '%s', got '%s'", i, expectedLetter, actual)
			}
		}
	})

	t.Run("Edge case: exactly 15% overhead", func(t *testing.T) {
		diskCapacity := int64(100 * 1024 * 1024 * 1024)      // 100GB
		guestCapacity := int64(float64(diskCapacity) * 0.85) // Exactly 15% overhead

		vm := model.VM{
			Disks: []model.Disk{
				{Capacity: diskCapacity},
			},
			GuestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: guestCapacity},
			},
		}

		adapter := &VmAdapter{model: vm}
		adapter.updateWinDriveLetters()

		if adapter.model.Disks[0].WinDriveLetter != "c" {
			t.Errorf("Disk should have drive letter 'c', got '%s'", adapter.model.Disks[0].WinDriveLetter)
		}
	})

	t.Run("Edge case: just over 15% overhead", func(t *testing.T) {
		diskCapacity := int64(100 * 1024 * 1024 * 1024)      // 100GB
		guestCapacity := int64(float64(diskCapacity) * 0.84) // Just over 15% overhead (16%)

		vm := model.VM{
			Disks: []model.Disk{
				{Capacity: diskCapacity},
			},
			GuestDisks: []model.GuestDisk{
				{DiskPath: "C:\\", Capacity: guestCapacity},
				{DiskPath: "D:\\", Capacity: 50 * 1024 * 1024 * 1024}, // Different disk
			},
		}

		adapter := &VmAdapter{model: vm}
		adapter.updateWinDriveLetters()

		// Should fall back to index-based matching since capacity is out of tolerance
		if adapter.model.Disks[0].WinDriveLetter != "c" {
			t.Errorf("Disk should have drive letter 'c', got '%s'", adapter.model.Disks[0].WinDriveLetter)
		}
	})
}

func TestUpdateWinDriveLetters_Performance(t *testing.T) {
	// Test with a larger number of disks to ensure the algorithm performs well
	const numDisks = 16

	disks := make([]model.Disk, numDisks)
	guestDisks := make([]model.GuestDisk, numDisks)

	for i := 0; i < numDisks; i++ {
		diskCapacity := int64((i + 1) * 1024 * 1024 * 1024)  // 1GB, 2GB, 3GB, etc.
		guestCapacity := int64(float64(diskCapacity) * 0.95) // 5% overhead

		disks[i] = model.Disk{Capacity: diskCapacity}

		// Use different drive letters to make each guest disk unique
		driveLetter := string(rune('C' + (i % 24))) // C, D, E, ..., Z, then wrap around
		guestDisks[i] = model.GuestDisk{
			DiskPath: driveLetter + ":\\",
			Capacity: guestCapacity,
		}
	}

	vm := model.VM{
		Disks:      disks,
		GuestDisks: guestDisks,
	}

	adapter := &VmAdapter{model: vm}

	// This should complete without timing out
	adapter.updateWinDriveLetters()

	// Verify all disks got matched with their corresponding drive letters
	for i := 0; i < numDisks; i++ {
		expectedLetter := strings.ToLower(string(rune('C' + (i % 24))))
		if adapter.model.Disks[i].WinDriveLetter != expectedLetter {
			t.Errorf("Disk %d should have drive letter '%s', got '%s'", i, expectedLetter, adapter.model.Disks[i].WinDriveLetter)
		}
	}
}
