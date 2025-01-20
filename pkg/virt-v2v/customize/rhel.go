package customize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/global"
	"github.com/konveyor/forklift-controller/pkg/virt-v2v/utils"
)

func CustomizeLinux(execFunc DomainExecFunc, disks []string, dir string, t FileSystemTool) error {
	var extraArgs []string

	// Step 1: Create files from the filesystem
	if err := t.CreateFilesFromFS(dir); err != nil {
		return fmt.Errorf("failed to create files from filesystem: %w", err)
	}

	// Step 2: Handle static IP configuration
	if err := handleStaticIPConfiguration(&extraArgs, dir); err != nil {
		return err
	}

	// Step 3: Add dynamic scripts from the configmap
	if _, err := os.Stat(global.DYNAMIC_SCRIPTS_MOUNT_PATH); !os.IsNotExist(err) {
		fmt.Println("Adding linux dynamic scripts")
		if err = addRhelDynamicScripts(&extraArgs, global.DYNAMIC_SCRIPTS_MOUNT_PATH); err != nil {
			return err
		}
	}

	// Step 4: Add scripts from embedded FS
	if err := addRhelRunScripts(&extraArgs, dir); err != nil {
		return err
	}
	if err := addRhelFirstbootScripts(&extraArgs, dir); err != nil {
		return err
	}

	// Step 5: Add the disks to customize
	addDisksToCustomize(&extraArgs, disks)

	// Step 6: Adds LUKS keys, if they exist
	if err := addLuksKeysToCustomize(&extraArgs); err != nil {
		return err
	}

	// Step 7: Execute the customization with the collected arguments
	if err := execFunc(extraArgs...); err != nil {
		return fmt.Errorf("failed to execute domain customization: %w", err)
	}

	return nil
}

// handleStaticIPConfiguration processes the static IP configuration and returns the initial extraArgs
func handleStaticIPConfiguration(extraArgs *[]string, dir string) error {
	envStaticIPs := os.Getenv("V2V_staticIPs")
	if envStaticIPs != "" {
		macToIPFilePath := filepath.Join(dir, "macToIP")
		macToIPFileContent := strings.ReplaceAll(envStaticIPs, "_", "\n") + "\n"

		if err := os.WriteFile(macToIPFilePath, []byte(macToIPFileContent), 0755); err != nil {
			return fmt.Errorf("failed to write MAC to IP mapping file: %w", err)
		}

		*extraArgs = append(*extraArgs, "--upload", macToIPFilePath+":/tmp/macToIP")
	}

	return nil
}

// addRhelFirstbootScripts appends firstboot script arguments to extraArgs
func addRhelFirstbootScripts(extraArgs *[]string, dir string) error {
	firstbootScriptsPath := filepath.Join(dir, "scripts", "rhel", "firstboot")

	firstBootScripts, err := getScriptsWithSuffix(firstbootScriptsPath, global.SHELL_SUFFIX)
	if err != nil {
		return err
	}

	if len(firstBootScripts) == 0 {
		fmt.Println("No run scripts found in directory:", firstbootScriptsPath)
		return nil
	}

	*extraArgs = append(*extraArgs, utils.GetScriptArgs("firstboot", firstBootScripts...)...)
	return nil
}

// addRhelRunScripts appends run script arguments to extraArgs
func addRhelRunScripts(extraArgs *[]string, dir string) error {
	runScriptsPath := filepath.Join(dir, "scripts", "rhel", "run")

	runScripts, err := getScriptsWithSuffix(runScriptsPath, global.SHELL_SUFFIX)
	if err != nil {
		return err
	}

	if len(runScripts) == 0 {
		fmt.Println("No run scripts found in directory:", runScriptsPath)
		return nil
	}

	*extraArgs = append(*extraArgs, utils.GetScriptArgs("run", runScripts...)...)
	return nil
}

// addLuksKeysToCustomize appends key arguments to extraArgs
func addLuksKeysToCustomize(extraArgs *[]string) error {
	luksArgs, err := utils.AddLUKSKeys()
	if err != nil {
		return fmt.Errorf("error adding LUKS kyes: %w", err)
	}
	*extraArgs = append(*extraArgs, luksArgs...)

	return nil
}

func addRhelDynamicScripts(extraArgs *[]string, dir string) error {
	dynamicScripts, err := getScriptsWithRegex(dir, global.LINUX_DYNAMIC_REGEX)
	if err != nil {
		return err
	}
	for _, script := range dynamicScripts {
		fmt.Printf("Adding linux dynamic scripts '%s'\n", script)
		r := regexp.MustCompile(global.LINUX_DYNAMIC_REGEX)
		groups := r.FindStringSubmatch(filepath.Base(script))
		// Option from the second regex group `(run|firstboot)`
		action := groups[2]
		*extraArgs = append(*extraArgs, utils.GetScriptArgs(action, script)...)
	}
	return nil
}
