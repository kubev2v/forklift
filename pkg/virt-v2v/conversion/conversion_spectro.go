package conversion

import (
	"fmt"
	"io"
	"os"
)

// prepareDiskFilesForInPlace ensures disk files are ready for virt-v2v-in-place.
// virt-v2v-in-place reads VM data from vSphere (via libvirt connection) and writes
// RAW format directly to the disk files specified in the libvirt XML. The files should
// be empty or not exist - if they contain invalid/corrupted VMDK data from a previous
// failed attempt, virt-v2v-in-place or qemu-img (used internally) may fail when trying
// to detect or process the file format.
func (c *Conversion) prepareDiskFilesForInPlace() error {
	for _, disk := range c.Disks {
		if disk.IsBlockDev {
			// Block devices don't need preparation
			continue
		}

		// Check if file exists
		info, err := os.Stat(disk.Path)
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist - that's fine, virt-v2v-in-place will create it
				continue
			}
			return fmt.Errorf("failed to stat disk file %s: %v", disk.Path, err)
		}

		// File exists - check if it's empty or has invalid content
		if info.Size() > 0 {
			// Try to detect if it's a valid image format
			// If it's corrupted VMDK or invalid data, truncate it
			// Open the file and check first few bytes
			file, err := os.Open(disk.Path)
			if err != nil {
				return fmt.Errorf("failed to open disk file %s: %v", disk.Path, err)
			}
			defer file.Close()

			// Read first 512 bytes to check format
			buf := make([]byte, 512)
			n, err := file.Read(buf)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to read disk file %s: %v", disk.Path, err)
			}

			// Check if it looks like VMDK (VMDK descriptor starts with "# Disk DescriptorFile")
			// or if it's corrupted/invalid data
			isVMDK := false
			if n >= 20 {
				header := string(buf[:min(20, n)])
				if len(header) >= 20 && header[:20] == "# Disk DescriptorFile" {
					isVMDK = true
				}
			}

			// If it's VMDK or we can't determine the format, truncate it
			// virt-v2v-in-place will write RAW format directly
			if isVMDK || n < 512 {
				fmt.Printf("Truncating disk file %s (size: %d) to prepare for in-place conversion\n", disk.Path, info.Size())
				err = os.Truncate(disk.Path, 0)
				if err != nil {
					return fmt.Errorf("failed to truncate disk file %s: %v", disk.Path, err)
				}
			}
			// If it's already RAW format and valid, leave it as is
		}
		// If file is empty (size 0), that's perfect - virt-v2v-in-place will write to it
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

