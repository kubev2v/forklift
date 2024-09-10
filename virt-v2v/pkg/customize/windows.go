package customize

import (
	"fmt"
	"path/filepath"

	"github.com/konveyor/forklift-controller/virt-v2v/pkg/utils"
)

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
func CustomizeWindows(execFunc DomainExecFunc, disks []string, dir string, t FileSystemTool) error {
	fmt.Printf("Customizing disks '%s'", disks)
	err := t.CreateFilesFromFS(dir)
	if err != nil {
		return err
	}
	windowsScriptsPath := filepath.Join(dir, "scripts", "windows")
	initPath := filepath.Join(windowsScriptsPath, "9999-restore_config_init.bat")
	restoreScriptPath := filepath.Join(windowsScriptsPath, "9999-restore_config.ps1")
	firstbootPath := filepath.Join(windowsScriptsPath, "firstboot.bat")

	// Upload scripts to the windows
	uploadScriptPath := fmt.Sprintf("%s:%s", restoreScriptPath, WIN_FIRSTBOOT_SCRIPTS_PATH)
	uploadInitPath := fmt.Sprintf("%s:%s", initPath, WIN_FIRSTBOOT_SCRIPTS_PATH)
	uploadFirstbootPath := fmt.Sprintf("%s:%s", firstbootPath, WIN_FIRSTBOOT_PATH)

	var extraArgs []string
	extraArgs = append(extraArgs, utils.GetScriptArgs("upload", uploadScriptPath, uploadInitPath, uploadFirstbootPath)...)
	extraArgs = append(extraArgs, utils.GetScriptArgs("add", disks...)...)
	err = execFunc(extraArgs...)
	if err != nil {
		return err
	}
	return nil
}
