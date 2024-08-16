package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed scripts
var scriptFS embed.FS

const (
	WIN_FIRSTBOOT_SCRIPTS_PATH = "/Program\\ Files/Guestfs/Firstboot/scripts"
)

// CustomizeWindowsImage customizes a windows disk image by uploading and setting firstboot batch scripts.
//
// The function writes two bash scripts to the specified local tmp directory,
// uploads them to the disk image using `virt-customize`, and sets the `init.bat` to run on the first boot
// of the image. The `virt-customize` currently supports only batch scripts for windows, so we execute our powershell
// scripts inside the bash scripts. If any errors occur during these operations, the function returns an error.
//
// Arguments:
//   - diskPath (string): The path to the XML file.
//
// Returns:
//   - error: An error if the file cannot be read, or nil if successful.
func CustomizeWindowsImage(diskPath string) error {
	t := EmbedTool{filesystem: &scriptFS}
	err := t.CreateFilesFromFS(DIR)
	if err != nil {
		return err
	}
	windowsScriptsPath := filepath.Join(DIR, "scripts", "windows")
	initPath := filepath.Join(windowsScriptsPath, "init.bat")
	restoreScriptPath := filepath.Join(windowsScriptsPath, "restore_config.ps1")

	uploadPath := fmt.Sprintf("%s:%s", restoreScriptPath, WIN_FIRSTBOOT_SCRIPTS_PATH)

	var extraArgs []string
	extraArgs = append(extraArgs, getScriptArgs("firstboot", initPath)...)
	extraArgs = append(extraArgs, getScriptArgs("upload", uploadPath)...)

	err = CustomizeImage(diskPath, extraArgs...)
	if err != nil {
		return err
	}
	return nil
}

// getScriptArgs generates a list of arguments.
//
// Arguments:
//   - argName (string): Argument name which should be used for all the values
//   - values (...string): The list of values which should be joined with argument names.
//
// Returns:
//   - []string: List of arguments
//
// Example:
//   - getScriptArgs("firstboot", boot1, boot2) => ["--firstboot", boot1, "--firstboot", boot2]
func getScriptArgs(argName string, values ...string) []string {
	var args []string
	for _, val := range values {
		args = append(args, fmt.Sprintf("--%s", argName), val)
	}
	return args
}

// CustomizeImage executes `virt-customize` to customize the image.
//
// Arguments:
//   - diskPath (string): The path to the disk image that is being customized.
//   - extraArgs (...string): The additional arguments which will be appended to the `virt-customize` arguments.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeImage(diskPath string, extraArgs ...string) error {
	args := []string{"--verbose", "-a", diskPath}
	args = append(args, extraArgs...)
	customizeCmd := exec.Command("virt-customize", args...)

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
