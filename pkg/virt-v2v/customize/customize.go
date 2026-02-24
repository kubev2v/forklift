package customize

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

const (
	WinFirstbootPath        = "/Program Files/Guestfs/Firstboot"
	WinFirstbootScriptsPath = "/Program Files/Guestfs/Firstboot/scripts"
	WindowsDynamicRegex     = `^([0-9]+_win_firstboot(([\w\-]*).ps1))$`
	LinuxDynamicRegex       = `^([0-9]+_linux_(run|firstboot)(([\w\-]*).sh))$`
	ShellSuffix             = ".sh"
	UploadCmd               = "--upload"
	RunCmd                  = "--run"
	FirstbootCmd            = "--firstboot"
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

type IPConfig struct {
	MAC string
	IPs []IPEntry
}

type IPEntry struct {
	IP           string
	Gateway      string
	PrefixLength string
	DNS          []string
}

type ScriptMatch struct {
	Path   string
	Groups []string
}

func formatIPs(ips []IPEntry) string {
	var b strings.Builder
	b.WriteString("(\n")
	for i, ip := range ips {
		b.WriteString("'" + ip.IP + "'")
		if i < len(ips)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(")")
	return b.String()
}

func formatDNS(dns []string) string {
	var b strings.Builder
	b.WriteString("(\n")
	for i, ip := range dns {
		b.WriteString("'" + ip + "'")
		if i < len(dns)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(")")
	return b.String()
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

// In case of multiple IP's per NIC on windows there is an existing setup script that assign only primary IP's
// With this function and its corresponding template we will inject all the complementry IP's to the NICs
func (c *Customize) injectComplementryStaticIPTemplate(templatePath, outputPath string) error {

	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	segments := strings.Split(c.appConfig.StaticIPs, "_")
	macMap := map[string][]IPEntry{}

	for _, segment := range segments {
		parts := strings.SplitN(segment, ":ip:", 2)
		if len(parts) != 2 {
			continue
		}
		mac := strings.ReplaceAll(parts[0], ":", "-") // Windows format
		ipParts := strings.Split(parts[1], ",")
		if len(ipParts) < 5 {
			continue
		}

		ip := ipParts[0]
		gw := ipParts[1]
		prefix := ipParts[2]
		dns := ipParts[3:]

		ipEntry := IPEntry{
			IP:           ip,
			Gateway:      gw,
			PrefixLength: prefix,
			DNS:          dns,
		}
		macMap[mac] = append(macMap[mac], ipEntry)
	}

	var configs []IPConfig
	for mac, ips := range macMap {
		if len(ips) > 1 {
			configs = append(configs, IPConfig{MAC: mac, IPs: ips[1:]}) // Skip the first (primary) IP
		}
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"add": func(a, b int) int {
			return a + b
		},
		"len": func(v interface{}) int {
			return reflect.ValueOf(v).Len()
		},
		"formatIPs": formatIPs,
		"formatDNS": func(cfg IPConfig) string {
			if len(cfg.IPs) > 0 {
				return formatDNS(cfg.IPs[0].DNS)
			}
			return "()"
		},
	}

	tmpl, err := template.New("preserveComplementryStaticIpScript").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, configs); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output script: %w", err)
	}

	return nil
}

func (c *Customize) injectStaticIPTemplate(templatePath, outputPath string) error {
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	tmpl, err := template.New("netConfigScript").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		InputString string
	}{
		InputString: c.appConfig.StaticIPs,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output script: %w", err)
	}

	return nil
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

	// Upload scripts to the windows
	uploadPreserveIpPath := ""
	uploadRemoveDuplicatesPath := ""
	uploadPreserveMultipleIpPath := ""
	if c.appConfig.VirtIoWinLegacyDrivers != "" {
		initPath = filepath.Join(windowsScriptsPath, "9999-run-mtv-ps-scripts-legacy.bat")

		if c.appConfig.StaticIPs != "" {
			networkConfigtemplate := filepath.Join(windowsScriptsPath, "9999-network-config.ps1.tmpl")
			networkConfigScript := filepath.Join(windowsScriptsPath, "9999-network-config.ps1")

			err := c.injectStaticIPTemplate(networkConfigtemplate, networkConfigScript)
			if err != nil {
				fmt.Printf("Error injecting static IP template: %v", err)
			}
			uploadPreserveIpPath = c.formatUpload(networkConfigScript, WinFirstbootScriptsPath)
		}
	}

	if c.appConfig.StaticIPs != "" {
		removeDuplicatesPersistentRoutesPath := filepath.Join(windowsScriptsPath, "9999-remove_duplicate_persistent_routes.ps1")
		uploadRemoveDuplicatesPath = c.formatUpload(removeDuplicatesPersistentRoutesPath, WinFirstbootScriptsPath)

		if c.appConfig.MultipleIpsPerNicName != "" {
			preserveIpsTemplate := filepath.Join(windowsScriptsPath, "9999-preserve_complementry_ips_per_nic.ps1.tmpl")
			preserveMultipleNicsPath := filepath.Join(windowsScriptsPath, "9999-preserve_complementry_ips_per_nic.ps1")
			err := c.injectComplementryStaticIPTemplate(preserveIpsTemplate, preserveMultipleNicsPath)
			if err != nil {
				fmt.Printf("Error injecting Complementry StaticIP's template: %v", err)
			}
			uploadPreserveMultipleIpPath = c.formatUpload(preserveMultipleNicsPath, WinFirstbootScriptsPath)
		}
	}
	uploadInitPath := c.formatUpload(initPath, WinFirstbootScriptsPath)
	cmdBuilder.AddArgs("--upload", uploadPreserveIpPath, uploadInitPath, uploadRemoveDuplicatesPath, uploadPreserveMultipleIpPath)
}

func (c *Customize) addWinDynamicScripts(cmdBuilder utils.CommandBuilder, dir string) error {
	dynamicScripts, err := c.getScriptsWithRegex(dir, WindowsDynamicRegex)
	if err != nil {
		return err
	}
	for _, script := range dynamicScripts {
		fmt.Printf("Adding windows dynamic scripts '%s'\n", script.Path)
		upload := c.formatUpload(script.Path, filepath.Join(WinFirstbootScriptsPath, filepath.Base(script.Path)))
		cmdBuilder.AddArg(UploadCmd, upload)
	}
	return nil
}

// getScriptsWithRegex retrieves all scripts matching the regex from the specified directory
// and returns both the script paths and their regex match groups
func (c *Customize) getScriptsWithRegex(directory string, regex string) ([]ScriptMatch, error) {
	files, err := c.fileSystem.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	r := regexp.MustCompile(regex)
	var scripts []ScriptMatch
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		groups := r.FindStringSubmatch(file.Name())
		if groups != nil {
			scriptPath := filepath.Join(directory, file.Name())
			scripts = append(scripts, ScriptMatch{
				Path:   scriptPath,
				Groups: groups,
			})
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
		cmdBuilder.AddArg(UploadCmd, fmt.Sprintf("%s:/tmp/macToIP", macToIPFilePath))
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
		cmdBuilder.AddArg(FirstbootCmd, scripts)
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
		cmdBuilder.AddArg(RunCmd, scripts)
	}
	return nil
}

// addLuksKeysToCustomize appends key arguments to extraArgs
func (c *Customize) addLuksKeysToCustomize(cmdBuilder utils.CommandBuilder) error {
	if c.appConfig.NbdeClevis {
		cmdBuilder.AddArgs("--key", "all:clevis")
		return nil
	}
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
		fmt.Printf("Adding linux dynamic scripts '%s'\n", script.Path)
		// Option from the second regex group `(run|firstboot)`
		action := script.Groups[2]
		var cmd string
		switch action {
		case "run":
			cmd = RunCmd
		case "firstboot":
			cmd = FirstbootCmd
		default:
			return fmt.Errorf("invalid action '%s' extracted from script filename '%s': expected 'run' or 'firstboot'", action, script.Path)
		}
		cmdBuilder.AddArg(cmd, script.Path)
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
