package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed scripts
var scriptFS embed.FS

const (
	WIN_FIRSTBOOT_PATH         = "/Program Files/Guestfs/Firstboot"
	WIN_FIRSTBOOT_SCRIPTS_PATH = "/Program Files/Guestfs/Firstboot/scripts"
)

// CustomizeWindows customizes a windows disk image by uploading scripts.
//
// The function writes two bash scripts to the specified local tmp directory,
// uploads them to the disk image using `virt-customize`.
//
// Arguments:
//   - disks ([]string): The list of disk paths which should be customized
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeWindows(disks []string) error {
	fmt.Printf("Customizing disks '%s'", disks)
	t := EmbedTool{filesystem: &scriptFS}
	err := t.CreateFilesFromFS(DIR)
	if err != nil {
		return err
	}
	windowsScriptsPath := filepath.Join(DIR, "scripts", "windows")
	initPath := filepath.Join(windowsScriptsPath, "9999-restore_config_init.bat")
	restoreScriptPath := filepath.Join(windowsScriptsPath, "9999-restore_config.ps1")
	firstbootPath := filepath.Join(windowsScriptsPath, "firstboot.bat")

	// Upload scripts to the windows
	uploadScriptPath := fmt.Sprintf("%s:%s", restoreScriptPath, WIN_FIRSTBOOT_SCRIPTS_PATH)
	uploadInitPath := fmt.Sprintf("%s:%s", initPath, WIN_FIRSTBOOT_SCRIPTS_PATH)
	uploadFirstbootPath := fmt.Sprintf("%s:%s", firstbootPath, WIN_FIRSTBOOT_PATH)

	var extraArgs []string
	extraArgs = append(extraArgs, getScriptArgs("upload", uploadScriptPath, uploadInitPath, uploadFirstbootPath)...)
	extraArgs = append(extraArgs, getScriptArgs("add", disks...)...)
	err = CustomizeDomainExec(extraArgs...)
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

// CustomizeDomainExec executes `virt-customize` to customize the image.
//
// Arguments:
//   - extraArgs (...string): The additional arguments which will be appended to the `virt-customize` arguments.
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func CustomizeDomainExec(extraArgs ...string) error {
	args := []string{"--verbose"}
	args = append(args, extraArgs...)
	customizeCmd := exec.Command("virt-customize", args...)
	customizeCmd.Stdout = os.Stdout
	customizeCmd.Stderr = os.Stderr

	fmt.Println("exec:", customizeCmd)
	if err := customizeCmd.Run(); err != nil {
		return fmt.Errorf("error executing virt-customize command: %w", err)
	}
	return nil
}
