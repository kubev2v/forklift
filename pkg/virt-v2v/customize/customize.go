package customize

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/config"
	"github.com/konveyor/forklift-controller/pkg/virt-v2v/utils"
)

const (
	WinFirstbootPath        = "/Program Files/Guestfs/Firstboot"
	WinFirstbootScriptsPath = "/Program Files/Guestfs/Firstboot/scripts"
	WindowsDynamicRegex     = `^([0-9]+_win_firstboot(([\w\-]*).ps1))$`
	LinuxDynamicRegex       = `^([0-9]+_linux_(run|firstboot)(([\w\-]*).sh))$`
	ShellSuffix             = ".sh"
)

//go:embed scripts
var scriptFS embed.FS

type Customize struct {
	disks           []string
	operatingSystem utils.InspectionOS
	appConfig       *config.AppConfig
	// Used for injecting mock to the builder
	commandBuilder     utils.CommandBuilder
	fileSystem         utils.FileSystem
	embeddedFileSystem EmbedTool
}

func NewCustomize(cfg *config.AppConfig, disks []string, operatingSystem utils.InspectionOS) *Customize {
	return &Customize{
		appConfig:          cfg,
		disks:              disks,
		operatingSystem:    operatingSystem,
		commandBuilder:     &utils.CommandBuilderImpl{},
		fileSystem:         &utils.FileSystemImpl{},
		embeddedFileSystem: &EmbedToolImpl{Filesystem: &scriptFS},
	}
}

func (c *Customize) Run() (err error) {
	fmt.Printf("Customizing disks '%s'\n", c.disks)
	// Customization for vSphere source.
	err = c.embeddedFileSystem.CreateFilesFromFS(c.appConfig.Workdir)
	if err != nil {
		return fmt.Errorf("failed to create files from filesystem: %w", err)
	}

	// windows
	if c.operatingSystem.IsWindows() {
		err = c.customizeWindows()
		if err != nil {
			fmt.Println("Error customizing disk image:", err)
			return err
		}
	}

	// Linux
	if !c.operatingSystem.IsWindows() {
		err = c.customizeLinux()
		if err != nil {
			fmt.Println("Error customizing disk image:", err)
			return err
		}
	}
	return nil
}

// customizeWindows customizes a windows disk image by uploading scripts.
//
// The function writes two bash scripts to the specified local tmp directory,
// uploads them to the disk image using `virt-customize`.
//
// Arguments:
//   - disks ([]string): The list of disk paths which should be customized
//
// Returns:
//   - error: An error if something goes wrong during the process, or nil if successful.
func (c *Customize) customizeWindows() (err error) {
	cmdBuilder := c.commandBuilder.New("virt-customize")
	cmdBuilder.AddFlag("--verbose")
	cmdBuilder.AddArg("--format", "raw")

	if _, err = c.fileSystem.Stat(c.appConfig.DynamicScriptsDir); !os.IsNotExist(err) {
		fmt.Println("Adding windows dynamic scripts")
		err = c.addWinDynamicScripts(cmdBuilder, c.appConfig.DynamicScriptsDir)
		if err != nil {
			return err
		}
	}

	c.addWinFirstbootScripts(cmdBuilder)

	c.addDisksToCustomize(cmdBuilder)

	err = c.runCmd(cmdBuilder)
	if err != nil {
		return err
	}
	return nil
}

// addDisksToCustomize appends disk arguments to extraArgs
func (c *Customize) addDisksToCustomize(cmdBuilder utils.CommandBuilder) {
	for _, disk := range c.disks {
		cmdBuilder.AddArg("--add", disk)
	}
}

func (c *Customize) runCmd(builder utils.CommandBuilder) error {
	customizeCmd := builder.Build()

	customizeCmd.SetStdout(os.Stdout)
	customizeCmd.SetStderr(os.Stderr)

	if err := customizeCmd.Run(); err != nil {
		return fmt.Errorf("error executing virt-customize command: %w", err)
	}
	return nil
}

// addWinFirstbootScripts appends firstboot script arguments to extraArgs
func (c *Customize) addWinFirstbootScripts(cmdBuilder utils.CommandBuilder) {
	windowsScriptsPath := filepath.Join(c.appConfig.Workdir, "scripts", "windows")
	initPath := filepath.Join(windowsScriptsPath, "9999-run-mtv-ps-scripts.bat")
	restoreScriptPath := filepath.Join(windowsScriptsPath, "9999-restore_config.ps1")
	firstbootPath := filepath.Join(windowsScriptsPath, "firstboot.bat")

	// Upload scripts to the windows
	uploadScriptPath := c.formatUpload(restoreScriptPath, WinFirstbootScriptsPath)
	uploadInitPath := c.formatUpload(initPath, WinFirstbootScriptsPath)
	uploadFirstbootPath := c.formatUpload(firstbootPath, WinFirstbootPath)

	cmdBuilder.AddArgs("--upload", uploadScriptPath, uploadInitPath, uploadFirstbootPath)
}

func (c *Customize) addWinDynamicScripts(cmdBuilder utils.CommandBuilder, dir string) error {
	dynamicScripts, err := c.getScriptsWithRegex(dir, WindowsDynamicRegex)
	if err != nil {
		return err
	}
	for _, script := range dynamicScripts {
		fmt.Printf("Adding windows dynamic scripts '%s'\n", script)
		upload := c.formatUpload(script, filepath.Join(WinFirstbootScriptsPath, filepath.Base(script)))
		cmdBuilder.AddArg("--upload", upload)
	}
	return nil
}

// getScriptsWithRegex retrieves all scripts with suffix from the specified directory
func (c *Customize) getScriptsWithRegex(directory string, regex string) ([]string, error) {
	files, err := c.fileSystem.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	r := regexp.MustCompile(regex)
	var scripts []string
	for _, file := range files {
		if !file.IsDir() && r.MatchString(file.Name()) {
			scriptPath := filepath.Join(directory, file.Name())
			scripts = append(scripts, scriptPath)
		}
	}
	return scripts, nil
}

func (c *Customize) customizeLinux() (err error) {
	cmdBuilder := c.commandBuilder.New("virt-customize")
	cmdBuilder.AddFlag("--verbose")
	cmdBuilder.AddArg("--format", "raw")

	// Step 2: Handle static IP configuration
	if err := c.handleStaticIPConfiguration(cmdBuilder); err != nil {
		return err
	}

	// Step 3: Add dynamic scripts from the configmap
	if _, err := c.fileSystem.Stat(c.appConfig.DynamicScriptsDir); !os.IsNotExist(err) {
		fmt.Println("Adding linux dynamic scripts")
		if err = c.addRhelDynamicScripts(cmdBuilder, c.appConfig.DynamicScriptsDir); err != nil {
			return err
		}
	}

	// Step 4: Add scripts from embedded FS
	if err := c.addRhelRunScripts(cmdBuilder); err != nil {
		return err
	}
	if err := c.addRhelFirstbootScripts(cmdBuilder); err != nil {
		return err
	}

	// Step 5: Add the disks to customize
	c.addDisksToCustomize(cmdBuilder)

	// Step 6: Adds LUKS keys, if they exist
	if err := c.addLuksKeysToCustomize(cmdBuilder); err != nil {
		return err
	}

	// Step 7: Execute the customization with the collected arguments
	if err := c.runCmd(cmdBuilder); err != nil {
		return fmt.Errorf("failed to execute domain customization: %w", err)
	}

	return nil
}

// handleStaticIPConfiguration processes the static IP configuration and returns the initial extraArgs
func (c *Customize) handleStaticIPConfiguration(cmdBuilder utils.CommandBuilder) error {
	if c.appConfig.StaticIPs != "" {
		macToIPFilePath := filepath.Join(c.appConfig.Workdir, "macToIP")
		macToIPFileContent := strings.ReplaceAll(c.appConfig.StaticIPs, "_", "\n") + "\n"

		if err := c.fileSystem.WriteFile(macToIPFilePath, []byte(macToIPFileContent), 0755); err != nil {
			return fmt.Errorf("failed to write MAC to IP mapping file: %w", err)
		}
		cmdBuilder.AddArg("--upload", fmt.Sprintf("%s:/tmp/macToIP", macToIPFilePath))
	}

	return nil
}

// addRhelFirstbootScripts appends firstboot script arguments to extraArgs
func (c *Customize) addRhelFirstbootScripts(cmdBuilder utils.CommandBuilder) error {
	firstbootScriptsPath := filepath.Join(c.appConfig.Workdir, "scripts", "rhel", "firstboot")

	firstBootScripts, err := c.getScriptsWithSuffix(firstbootScriptsPath, ShellSuffix)
	if err != nil {
		return err
	}

	if len(firstBootScripts) == 0 {
		fmt.Println("No run scripts found in directory:", firstbootScriptsPath)
		return nil
	}
	for _, scripts := range firstBootScripts {
		cmdBuilder.AddArg("--firstboot", scripts)
	}
	return nil
}

// addRhelRunScripts appends run script arguments to extraArgs
func (c *Customize) addRhelRunScripts(cmdBuilder utils.CommandBuilder) error {
	runScriptsPath := filepath.Join(c.appConfig.Workdir, "scripts", "rhel", "run")

	runScripts, err := c.getScriptsWithSuffix(runScriptsPath, ShellSuffix)
	if err != nil {
		return err
	}

	if len(runScripts) == 0 {
		fmt.Println("No run scripts found in directory:", runScriptsPath)
		return nil
	}
	for _, scripts := range runScripts {
		cmdBuilder.AddArg("--run", scripts)
	}
	return nil
}

// addLuksKeysToCustomize appends key arguments to extraArgs
func (c *Customize) addLuksKeysToCustomize(cmdBuilder utils.CommandBuilder) error {
	if c.appConfig.Luksdir == "" {
		return nil
	}
	err := utils.AddLUKSKeys(c.fileSystem, cmdBuilder, c.appConfig.Luksdir)
	if err != nil {
		return fmt.Errorf("error adding LUKS kyes: %w", err)
	}

	return nil
}

func (c *Customize) addRhelDynamicScripts(cmdBuilder utils.CommandBuilder, dir string) error {
	dynamicScripts, err := c.getScriptsWithRegex(dir, LinuxDynamicRegex)
	if err != nil {
		return err
	}
	for _, script := range dynamicScripts {
		fmt.Printf("Adding linux dynamic scripts '%s'\n", script)
		r := regexp.MustCompile(LinuxDynamicRegex)
		groups := r.FindStringSubmatch(filepath.Base(script))
		// Option from the second regex group `(run|firstboot)`
		action := groups[2]
		cmdBuilder.AddArg(action, script)
	}
	return nil
}

// getScriptsWithSuffix retrieves all scripts with suffix from the specified directory
func (c *Customize) getScriptsWithSuffix(directory string, suffix string) ([]string, error) {
	files, err := c.fileSystem.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	var scripts []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), suffix) && !strings.HasPrefix(file.Name(), "test-") {
			scriptPath := filepath.Join(directory, file.Name())
			scripts = append(scripts, scriptPath)
		}
	}

	return scripts, nil
}

func (c *Customize) formatUpload(src string, dst string) string {
	return fmt.Sprintf("%s:%s", src, dst)
}
