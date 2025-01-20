package customize

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/global"
	"github.com/konveyor/forklift-controller/pkg/virt-v2v/utils"
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
	err := t.CreateFilesFromFS(dir)
	if err != nil {
		return fmt.Errorf("failed to create files from filesystem: %w", err)
	}

	var extraArgs []string

	if _, err = os.Stat(global.DYNAMIC_SCRIPTS_MOUNT_PATH); !os.IsNotExist(err) {
		fmt.Println("Adding windows dynamic scripts")
		err = addWinDynamicScripts(&extraArgs, global.DYNAMIC_SCRIPTS_MOUNT_PATH)
		if err != nil {
			return err
		}
	}

	addWinFirstbootScripts(&extraArgs, dir)

	addDisksToCustomize(&extraArgs, disks)

	err = execFunc(extraArgs...)
	if err != nil {
		return err
	}
	return nil
}

// addRhelFirstbootScripts appends firstboot script arguments to extraArgs
func addWinFirstbootScripts(extraArgs *[]string, dir string) {
	windowsScriptsPath := filepath.Join(dir, "scripts", "windows")
	initPath := filepath.Join(windowsScriptsPath, "9999-run-mtv-ps-scripts.bat")
	restoreScriptPath := filepath.Join(windowsScriptsPath, "9999-restore_config.ps1")
	firstbootPath := filepath.Join(windowsScriptsPath, "firstboot.bat")

	// Upload scripts to the windows
	uploadScriptPath := formatUpload(restoreScriptPath, global.WIN_FIRSTBOOT_SCRIPTS_PATH)
	uploadInitPath := formatUpload(initPath, global.WIN_FIRSTBOOT_SCRIPTS_PATH)
	uploadFirstbootPath := formatUpload(firstbootPath, global.WIN_FIRSTBOOT_PATH)

	*extraArgs = append(*extraArgs, utils.GetScriptArgs("upload", uploadScriptPath, uploadInitPath, uploadFirstbootPath)...)
}

func addWinDynamicScripts(extraArgs *[]string, dir string) error {
	dynamicScripts, err := getScriptsWithRegex(dir, global.WINDOWS_DYNAMIC_REGEX)
	if err != nil {
		return err
	}
	for _, script := range dynamicScripts {
		fmt.Printf("Adding windows dynamic scripts '%s'\n", script)
		upload := formatUpload(script, filepath.Join(global.WIN_FIRSTBOOT_SCRIPTS_PATH, filepath.Base(script)))
		*extraArgs = append(*extraArgs, utils.GetScriptArgs("upload", upload)...)
	}
	return nil
}
