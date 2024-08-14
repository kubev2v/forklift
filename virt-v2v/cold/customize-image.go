package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// CustomizeImage customizes a disk image by uploading and setting firstboot bash scripts.
//
// The function writes two bash scripts to the specified local tmp directory,
// uploads them to the disk image using `virt-customize`, and sets them to run on the first boot
// of the image. If any errors occur during these operations, the function returns an error.
//
// Arguments:
//   - dir (string): The directory where the bash scripts will be stored locally.
//   - diskPath (string): The path to the disk image that is being customized.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeImage(dir string, diskPath string) error {
	checkConnectionBashFilePath := filepath.Join(dir, "check-connection.sh")
	err := WriteBashScript(CheckConnectivityBash, checkConnectionBashFilePath)
	if err != nil {
		return fmt.Errorf("failed to write check-connection bash script: %w", err)
	}

	copyConnectionsBashFilePath := filepath.Join(dir, "copy-connections.sh")
	err = WriteBashScript(CopyConnectionsBash, copyConnectionsBashFilePath)
	if err != nil {
		return fmt.Errorf("failed to write copy-connections bash script: %w", err)
	}

	customizeCmd := exec.Command("virt-customize", "--verbose",
		"-a", diskPath,
		"--firstboot", checkConnectionBashFilePath,
		"--firstboot", copyConnectionsBashFilePath)

	fmt.Println("exec:", customizeCmd)
	if err := customizeCmd.Run(); err != nil {
		return fmt.Errorf("error executing virt-customize command: %w", err)
	}

	return nil
}

// FindRootDiskImage locates the root disk image in the specified directory based on the disk name pattern.
//
// This function looks for disk files in the `dir` directory that match the pattern `*-sd*`. It uses the `V2V_RootDisk`
// environment variable to determine the disk name, falling back to the default disk name "sda" if the environment variable is not set or invalid.
//
// Arguments:
//   - dir (string): The directory containing the disk files.
//
// Returns:
//   - string: The full path to the root disk image.
//   - error: An error if the root disk image is not found or if there is an issue reading the disk directory.
func FindRootDiskImage(dir string) (string, error) {
	var err error

	// Get all disk files matching the pattern
	disks, err := filepath.Glob(filepath.Join(dir, "*-sd*"))
	if err != nil {
		return "", fmt.Errorf("error getting disks: %w", err)
	}

	// Default disk name
	diskName := "sda"

	// Check if V2V_RootDisk environment variable is set
	if v2vRootDisk := os.Getenv("V2V_RootDisk"); v2vRootDisk != "" {
		// Extract just the disk name, ignoring partition numbers if present
		trimmedDiskName := strings.TrimLeft(v2vRootDisk, "0123456789")

		// Ensure the disk name is of the form "sd[letter]"
		matched, _ := regexp.MatchString(`^sd[a-z]$`, trimmedDiskName)
		if matched {
			diskName = trimmedDiskName
		} else {
			fmt.Printf("Warning: Invalid disk name format '%s' in V2V_RootDisk. Using default 'sda'.\n", v2vRootDisk)
		}
	}

	// Search for a matching disk path
	var rootDiskPath string
	for _, disk := range disks {
		if strings.HasSuffix(disk, "-"+diskName) {
			rootDiskPath = disk
			break
		}
	}

	if rootDiskPath == "" {
		return "", fmt.Errorf("root disk not found for disk name: %s", diskName)
	}

	return rootDiskPath, nil
}
