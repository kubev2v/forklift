package util

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestKubevirtVmParser(t *testing.T) {
	fmt.Println("test")
	testFile(t, "new_format_bios.yml", "bios")
	testFile(t, "new_format_efi.yml", "uefi")
	testFile(t, "old_format_bios.yml", "bios")
	testFile(t, "old_format_efi.yml", "uefi")
	testFile(t, "old_format_none.yml", "")
	testFile(t, "new_format_none.yml", "")
}

func testFile(t *testing.T, filename, expectedFormat string) {
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		fmt.Println(err)
	}
	firmware, err := GetFirmwareFromYaml(data)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(firmware)
	if firmware != expectedFormat {
		t.Fatalf("Failed to parse '%s' from file '%s'", expectedFormat, filename)
	}
}

func TestGetDiskBootOrderFromYaml(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		inline    []byte
		expected  int
		expectErr bool
	}{
		{"boot disk on last disk", "new_format_boot_order.yml", nil, 2, false},
		{"boot disk on first disk", "new_format_boot_order_first_disk.yml", nil, 0, false},
		{"no boot order present", "new_format_bios.yml", nil, -1, false},
		{"invalid YAML", "", []byte("not valid yaml: ["), -1, true},
		{"empty YAML", "", []byte(""), -1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data []byte
			var err error
			if tc.file != "" {
				data, err = os.ReadFile(filepath.Join("testdata", tc.file))
				if err != nil {
					t.Fatalf("Failed to read test file: %v", err)
				}
			} else {
				data = tc.inline
			}

			bootDisk, err := GetDiskBootOrderFromYaml(data)
			if tc.expectErr && err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if bootDisk != tc.expected {
				t.Fatalf("Expected boot disk index %d, got %d", tc.expected, bootDisk)
			}
		})
	}
}
