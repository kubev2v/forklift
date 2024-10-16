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
