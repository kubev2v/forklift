package customize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konveyor/forklift-controller/virt-v2v/pkg/utils"
)

func CustomizeLinux(execFunc DomainExecFunc, disks []string, dir string, t FileSystemTool) error {
	fmt.Printf("Customizing disks '%v'\n", disks)

	var extraArgs []string

	// Step 1: Create files from the filesystem
	if err := t.CreateFilesFromFS(dir); err != nil {
		return fmt.Errorf("failed to create files from filesystem: %w", err)
	}

	// Step 2: Handle static IP configuration
	if err := handleStaticIPConfiguration(&extraArgs, dir); err != nil {
		return err
	}

	// Step 3: Add scripts
	if err := addRunScripts(&extraArgs, dir); err != nil {
		return err
	}
	if err := addFirstbootScripts(&extraArgs, dir); err != nil {
		return err
	}

	// Step 4: Add the disks to customize
	addDisksToCustomize(&extraArgs, disks)

	// Step 5: Adds LUKS keys, if they exist
	if err := addLuksKeysToCustomize(&extraArgs); err != nil {
		return err
	}

	// Step 6: Execute the customization with the collected arguments
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

// addFirstbootScripts appends firstboot script arguments to extraArgs
func addFirstbootScripts(extraArgs *[]string, dir string) error {
	firstbootScriptsPath := filepath.Join(dir, "scripts", "rhel", "firstboot")

	firstBootScripts, err := getScripts(firstbootScriptsPath)
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

// addRunScripts appends run script arguments to extraArgs
func addRunScripts(extraArgs *[]string, dir string) error {
	runScriptsPath := filepath.Join(dir, "scripts", "rhel", "run")

	runScripts, err := getScripts(runScriptsPath)
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

// getScripts retrieves all .sh scripts from the specified directory
func getScripts(directory string) ([]string, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read firstboot scripts directory: %w", err)
	}

	var scripts []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") && !strings.HasPrefix(file.Name(), "test-") {
			scriptPath := filepath.Join(directory, file.Name())
			scripts = append(scripts, scriptPath)
		}
	}

	return scripts, nil
}

// addDisksToCustomize appends disk arguments to extraArgs
func addDisksToCustomize(extraArgs *[]string, disks []string) {
	*extraArgs = append(*extraArgs, utils.GetScriptArgs("add", disks...)...)
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
