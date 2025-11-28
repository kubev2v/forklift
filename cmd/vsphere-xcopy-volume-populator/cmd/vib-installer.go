package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/soap"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

const (
	defaultVIBPath    = "/bin/vmkfstools-wrapper.vib"
	defaultSSHTimeout = 30
	tempVIBPath       = "/tmp/vmkfstools-wrapper.vib"
	colorReset        = "\033[0m"
	colorRed          = "\033[31m"
	colorGreen        = "\033[32m"
	colorYellow       = "\033[33m"
)

type Config struct {
	// VIB installation
	VIBPath      string
	SSHKeyFile   string
	SSHTimeout   int
	RestartHostd bool
	ESXiHosts    string // comma-separated list

	// vSphere discovery
	VCenterUsername string
	VCenterPassword string
	VCenterHostname string
	VCenterInsecure bool
	Datacenter      string // optional datacenter filter for host discovery
}

type InstallationResult struct {
	Host    string
	Success bool
	Error   error
}

func main() {
	config := &Config{}

	// Define flags (following vsphere-xcopy-volume-populator.go naming conventions)
	flag.StringVar(&config.VIBPath, "vib-path", getEnv("VIB_PATH", defaultVIBPath), "Path to VIB file")
	flag.StringVar(&config.SSHKeyFile, "ssh-key-file", getEnv("SSH_KEY_FILE", ""), "Path to SSH private key file")
	flag.StringVar(&config.ESXiHosts, "esxi-hosts", getEnv("ESXI_HOSTS", ""), "Comma-separated list of ESXi hosts")
	flag.IntVar(&config.SSHTimeout, "ssh-timeout-seconds", getEnvInt("SSH_TIMEOUT_SECONDS", defaultSSHTimeout), "SSH connection timeout in seconds")
	flag.BoolVar(&config.RestartHostd, "restart-hostd", getEnvBool("RESTART_HOSTD", true), "Restart hostd service after installation")

	// vSphere discovery flags (using same env vars as vsphere-xcopy-volume-populator.go)
	flag.StringVar(&config.VCenterUsername, "vsphere-username", os.Getenv("GOVMOMI_USERNAME"), "vSphere username for auto-discovery")
	flag.StringVar(&config.VCenterPassword, "vsphere-password", os.Getenv("GOVMOMI_PASSWORD"), "vSphere password for auto-discovery")
	flag.StringVar(&config.VCenterHostname, "vsphere-hostname", os.Getenv("GOVMOMI_HOSTNAME"), "vSphere hostname for auto-discovery")
	flag.BoolVar(&config.VCenterInsecure, "vsphere-insecure", getEnvBool("GOVMOMI_INSECURE", false), "Skip SSL verification for vSphere")
	flag.StringVar(&config.Datacenter, "datacenter", "", "Datacenter name to filter ESXi hosts (optional, for auto-discovery)")

	klog.InitFlags(nil)
	flag.Parse()

	if err := validateConfig(config); err != nil {
		logError("Configuration error: %v", err)
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	// Get list of ESXi hosts
	hosts, err := getESXiHosts(ctx, config)
	if err != nil {
		logError("Failed to get ESXi hosts: %v", err)
		os.Exit(1)
	}

	if len(hosts) == 0 {
		logError("No ESXi hosts found")
		os.Exit(1)
	}

	logInfo("Installing VIB on %d host(s)...", len(hosts))
	logInfo("VIB path: %s", config.VIBPath)
	logInfo("SSH key: %s", config.SSHKeyFile)

	// Load SSH private key
	privateKey, err := os.ReadFile(config.SSHKeyFile)
	if err != nil {
		logError("Failed to read SSH key file: %v", err)
		os.Exit(1)
	}

	// Install on all hosts
	results := make([]InstallationResult, 0, len(hosts))
	for _, host := range hosts {
		result := installVIBOnHost(ctx, config, host, privateKey)
		results = append(results, result)
		fmt.Println() // Empty line between hosts
	}

	// Print summary
	printSummary(results)

	// Exit with error if any installation failed
	for _, result := range results {
		if !result.Success {
			os.Exit(1)
		}
	}
}

func validateConfig(config *Config) error {
	if config.SSHKeyFile == "" {
		return fmt.Errorf("SSH key file is required (--ssh-key-file or SSH_KEY_FILE)")
	}

	if _, err := os.Stat(config.SSHKeyFile); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file not found: %s", config.SSHKeyFile)
	}

	if _, err := os.Stat(config.VIBPath); os.IsNotExist(err) {
		return fmt.Errorf("VIB file not found: %s", config.VIBPath)
	}

	if config.ESXiHosts == "" && (config.VCenterUsername == "" || config.VCenterPassword == "" || config.VCenterHostname == "") {
		return fmt.Errorf("either --esxi-hosts or vCenter credentials are required")
	}

	return nil
}

func getESXiHosts(ctx context.Context, config *Config) ([]string, error) {
	// If hosts are explicitly provided, use them
	if config.ESXiHosts != "" {
		hosts := strings.Split(config.ESXiHosts, ",")
		trimmedHosts := make([]string, 0, len(hosts))
		for _, host := range hosts {
			trimmed := strings.TrimSpace(host)
			if trimmed != "" {
				trimmedHosts = append(trimmedHosts, trimmed)
			}
		}
		return trimmedHosts, nil
	}

	// Otherwise, discover from vCenter
	if config.VCenterUsername != "" && config.VCenterPassword != "" && config.VCenterHostname != "" {
		return discoverESXiHostsFromVCenter(ctx, config)
	}

	return nil, fmt.Errorf("no ESXi hosts specified and vCenter credentials not provided")
}

func discoverESXiHostsFromVCenter(ctx context.Context, config *Config) ([]string, error) {
	logInfo("Discovering ESXi hosts from vCenter...")

	// Create vCenter URL
	u, err := soap.ParseURL("https://" + config.VCenterHostname + "/sdk")
	if err != nil {
		return nil, err
	}
	u.User = url.UserPassword(config.VCenterUsername, config.VCenterPassword)

	// Connect to vCenter
	client, err := govmomi.NewClient(ctx, u, config.VCenterInsecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vCenter: %w", err)
	}
	defer client.Logout(ctx)

	// Find all hosts
	finder := find.NewFinder(client.Client, true)

	// Set datacenter if specified
	var searchPath string
	if config.Datacenter != "" {
		logInfo("Filtering hosts by datacenter: %s", config.Datacenter)
		dc, err := finder.Datacenter(ctx, config.Datacenter)
		if err != nil {
			return nil, fmt.Errorf("failed to find datacenter %s: %w", config.Datacenter, err)
		}
		finder.SetDatacenter(dc)
		searchPath = "*"
	} else {
		searchPath = "*"
	}

	hosts, err := finder.HostSystemList(ctx, searchPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list ESXi hosts: %w", err)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no ESXi hosts found in vCenter")
	}

	hostIPs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		ip, err := vmware.GetHostIPAddress(ctx, host)
		if err != nil {
			logWarn("Failed to get IP for host %s: %v", host.Name(), err)
			continue
		}
		hostIPs = append(hostIPs, ip)
		logInfo("  Discovered: %s (%s)", host.Name(), ip)
	}

	return hostIPs, nil
}

func installVIBOnHost(ctx context.Context, config *Config, host string, privateKey []byte) InstallationResult {
	logInfo("[%s] Starting installation...", host)

	// Parse private key
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		logError("[%s] Failed to parse private key: %v", host, err)
		return InstallationResult{Host: host, Success: false, Error: err}
	}

	// SSH configuration
	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(config.SSHTimeout) * time.Second,
	}

	// Connect to ESXi host
	addr := fmt.Sprintf("%s:22", host)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		logError("[%s] SSH connection failed: %v", host, err)
		return InstallationResult{Host: host, Success: false, Error: err}
	}
	defer client.Close()

	logInfo("[%s] SSH connection successful", host)

	// Copy VIB file to ESXi host
	logInfo("[%s] Copying VIB file...", host)
	if err := copyFileToHost(client, config.VIBPath, tempVIBPath); err != nil {
		logError("[%s] Failed to copy VIB file: %v", host, err)
		return InstallationResult{Host: host, Success: false, Error: err}
	}

	// Check if VIB is already installed
	logInfo("[%s] Checking for existing installation...", host)
	if isInstalled, err := checkVIBInstalled(client); err == nil && isInstalled {
		logWarn("[%s] VIB already installed, removing old version...", host)
		if err := removeVIB(client); err != nil {
			logWarn("[%s] Failed to remove old VIB, continuing anyway: %v", host, err)
		}
	}

	// Install VIB
	logInfo("[%s] Installing VIB...", host)
	if err := installVIB(client, tempVIBPath); err != nil {
		logError("[%s] VIB installation failed: %v", host, err)
		return InstallationResult{Host: host, Success: false, Error: err}
	}

	// Verify installation
	logInfo("[%s] Verifying installation...", host)
	if installed, err := checkVIBInstalled(client); err != nil || !installed {
		logError("[%s] VIB verification failed", host)
		return InstallationResult{Host: host, Success: false, Error: fmt.Errorf("verification failed")}
	}

	// Restart hostd service
	if config.RestartHostd {
		logInfo("[%s] Restarting hostd service...", host)
		if err := restartHostd(client); err != nil {
			logWarn("[%s] Failed to restart hostd: %v", host, err)
			logWarn("[%s] Please restart manually: /etc/init.d/hostd restart", host)
		} else {
			logInfo("[%s] Waiting for hostd to restart...", host)
			time.Sleep(10 * time.Second)
		}
	}

	// Test vmkfstools namespace
	logInfo("[%s] Testing vmkfstools namespace...", host)
	if err := testVmkfstoolsNamespace(client); err == nil {
		logSuccess("[%s] ✓ Installation successful! vmkfstools namespace is available.", host)
	} else {
		logWarn("[%s] ⚠ VIB installed but vmkfstools namespace not yet available", host)
		logWarn("[%s] You may need to restart hostd manually: /etc/init.d/hostd restart", host)
	}

	// Cleanup temp VIB file
	_, _ = runSSHCommand(client, fmt.Sprintf("rm -f %s", tempVIBPath))

	return InstallationResult{Host: host, Success: true, Error: nil}
}

func copyFileToHost(client *ssh.Client, localPath, remotePath string) error {
	// Read local file
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Use cat to write the file directly
	cmd := fmt.Sprintf("cat > %s", remotePath)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Get stderr to capture any errors
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Write the file data directly
	if _, err := stdin.Write(data); err != nil {
		stdin.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Close stdin to signal EOF to cat
	stdin.Close()

	// Wait for command to complete
	if err := session.Wait(); err != nil {
		// Read any error output
		errOutput := make([]byte, 1024)
		n, _ := stderr.Read(errOutput)
		if n > 0 {
			return fmt.Errorf("failed to copy file: %w (stderr: %s)", err, string(errOutput[:n]))
		}
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func checkVIBInstalled(client *ssh.Client) (bool, error) {
	output, err := runSSHCommand(client, "esxcli software vib list | grep vmkfstools-wrapper")
	if err != nil {
		// grep returns non-zero exit code if not found
		return false, nil
	}
	return strings.Contains(output, "vmkfstools-wrapper"), nil
}

func removeVIB(client *ssh.Client) error {
	_, err := runSSHCommand(client, "esxcli software vib remove -n vmkfstools-wrapper")
	return err
}

func installVIB(client *ssh.Client, vibPath string) error {
	cmd := fmt.Sprintf("esxcli software vib install -v %s -f", vibPath)
	_, err := runSSHCommand(client, cmd)
	return err
}

func restartHostd(client *ssh.Client) error {
	_, err := runSSHCommand(client, "/etc/init.d/hostd restart")
	return err
}

func testVmkfstoolsNamespace(client *ssh.Client) error {
	_, err := runSSHCommand(client, "esxcli vmkfstools clone --help")
	return err
}

func runSSHCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

func printSummary(results []InstallationResult) {
	fmt.Println("========================================")
	logInfo("Installation Summary")
	fmt.Println("========================================")

	successCount := 0
	failureCount := 0
	failedHosts := make([]string, 0)

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
			failedHosts = append(failedHosts, result.Host)
		}
	}

	logInfo("Total hosts: %d", len(results))
	logInfo("Successful: %d", successCount)
	logInfo("Failed: %d", failureCount)

	if failureCount > 0 {
		fmt.Println()
		logError("Failed hosts:")
		for _, host := range failedHosts {
			fmt.Printf("  - %s\n", host)
		}
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func logInfo(format string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", colorGreen, colorReset, fmt.Sprintf(format, args...))
}

func logWarn(format string, args ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", colorYellow, colorReset, fmt.Sprintf(format, args...))
}

func logError(format string, args ...interface{}) {
	fmt.Printf("%s[ERROR]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
}

func logSuccess(format string, args ...interface{}) {
	fmt.Printf("%s%s%s\n", colorGreen, fmt.Sprintf(format, args...), colorReset)
}
