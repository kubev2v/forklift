package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	vspherelib "github.com/kubev2v/forklift/pkg/lib/client/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/client/vsphere/vmware"
	"github.com/vmware/govmomi/object"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

const (
	defaultVIBPath  = "/usr/local/share/forklift/vmkfstools-wrapper.vib"
	hostdRestartWait = 30 * time.Second
	colorReset      = "\033[0m"
	colorRed        = "\033[31m"
	colorGreen      = "\033[32m"
	colorYellow     = "\033[33m"
)

type Config struct {
	VIBPath      string
	SSHKeyFile   string
	SSHPassword  string
	SSHTimeout   int
	RestartHostd bool
	ESXiHosts    string // comma-separated list

	VCenterUsername string
	VCenterPassword string
	VCenterHostname string
	VCenterInsecure bool
}

type InstallationResult struct {
	Host    string
	Success bool
	Error   error
}

func main() {
	config := &Config{}

	flag.StringVar(&config.VIBPath, "vib-path", getEnv("VIB_PATH", defaultVIBPath), "Path to VIB file")
	flag.StringVar(&config.SSHKeyFile, "ssh-key-file", getEnv("SSH_KEY_FILE", ""), "Path to SSH private key file")
	flag.StringVar(&config.SSHPassword, "ssh-password", "", "SSH password for ESXi root (env: SSH_PASSWORD)")
	flag.StringVar(&config.ESXiHosts, "esxi-hosts", getEnv("ESXI_HOSTS", ""), "Comma-separated list of ESXi host IPs")
	flag.IntVar(&config.SSHTimeout, "ssh-timeout-seconds", getEnvInt("SSH_TIMEOUT_SECONDS", 30), "SSH connection timeout in seconds")
	flag.BoolVar(&config.RestartHostd, "restart-hostd", getEnvBool("RESTART_HOSTD", true), "Restart hostd service after installation")

	flag.StringVar(&config.VCenterUsername, "vsphere-username", "", "vSphere username (env: GOVMOMI_USERNAME)")
	flag.StringVar(&config.VCenterPassword, "vsphere-password", "", "vSphere password (env: GOVMOMI_PASSWORD)")
	flag.StringVar(&config.VCenterHostname, "vsphere-hostname", "", "vSphere hostname (env: GOVMOMI_HOSTNAME)")
	flag.BoolVar(&config.VCenterInsecure, "vsphere-insecure", getEnvBool("GOVMOMI_INSECURE", false), "Skip SSL verification for vSphere")

	klog.InitFlags(nil)
	flag.Parse()

	if config.VCenterUsername == "" {
		config.VCenterUsername = os.Getenv("GOVMOMI_USERNAME")
	}
	if config.VCenterPassword == "" {
		config.VCenterPassword = os.Getenv("GOVMOMI_PASSWORD")
	}
	if config.VCenterHostname == "" {
		config.VCenterHostname = os.Getenv("GOVMOMI_HOSTNAME")
	}
	if config.SSHPassword == "" {
		config.SSHPassword = os.Getenv("SSH_PASSWORD")
	}

	if err := validateConfig(config); err != nil {
		logError("Configuration error: %v", err)
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	vcenterURL := fmt.Sprintf("https://%s/sdk", config.VCenterHostname)
	if config.VCenterInsecure {
		vcenterURL = fmt.Sprintf("https://%s/sdk?insecure=true", config.VCenterHostname)
	}
	client, err := vmware.NewClient(vcenterURL, config.VCenterUsername, config.VCenterPassword)
	if err != nil {
		logError("Failed to connect to vCenter: %v", err)
		os.Exit(1)
	}
	defer client.Logout(ctx)

	hosts, err := discoverHosts(ctx, client, config.ESXiHosts)
	if err != nil {
		logError("Failed to discover hosts: %v", err)
		os.Exit(1)
	}

	if len(hosts) == 0 {
		logError("No ESXi hosts found")
		os.Exit(1)
	}

	logInfo("Installing VIB on %d host(s)...", len(hosts))
	logInfo("VIB path: %s", config.VIBPath)
	logInfo("Required version: %s", vspherelib.VibVersion)

	var sshConfig *ssh.ClientConfig
	if config.RestartHostd {
		var err error
		sshConfig, err = buildSSHConfig(config)
		if err != nil {
			logError("%v", err)
			os.Exit(1)
		}
	}

	installer := &vspherelib.DefaultVIBInstaller{}
	results := make([]InstallationResult, 0, len(hosts))

	for _, host := range hosts {
		hostIP, err := vmware.GetHostIPAddress(ctx, host)
		if err != nil {
			logError("[%s] Failed to get host IP: %v", host.Name(), err)
			results = append(results, InstallationResult{Host: host.Name(), Success: false, Error: err})
			continue
		}

		logInfo("[%s] (%s) Starting installation...", host.Name(), hostIP)

		if err := installer.InstallVib(ctx, client, host, config.VIBPath); err != nil {
			logError("[%s] VIB installation failed: %v", host.Name(), err)
			results = append(results, InstallationResult{Host: host.Name(), Success: false, Error: err})
			continue
		}
		logInfo("[%s] VIB installed on disk", host.Name())

		if config.RestartHostd {
			logInfo("[%s] Restarting hostd to load VIB into memory...", host.Name())
			if err := vspherelib.RestartHostd(ctx, hostIP, sshConfig); err != nil {
				logWarn("[%s] hostd restart failed: %v", host.Name(), err)
				logWarn("[%s] Restart manually: /etc/init.d/hostd restart", host.Name())
			} else {
				logInfo("[%s] Waiting for hostd to restart...", host.Name())
				time.Sleep(hostdRestartWait)
			}
		}

		loadedVersion, err := vspherelib.GetLoadedVIBVersion(ctx, client, host)
		if err != nil {
			logWarn("[%s] Could not verify loaded version: %v", host.Name(), err)
			logWarn("[%s] VIB installed but may need manual hostd restart", host.Name())
			results = append(results, InstallationResult{Host: host.Name(), Success: false, Error: fmt.Errorf("could not verify loaded VIB version: %w", err)})
		} else if loadedVersion == vspherelib.VibVersion {
			logSuccess("[%s] VIB %s installed and active", host.Name(), loadedVersion)
			results = append(results, InstallationResult{Host: host.Name(), Success: true})
		} else {
			logWarn("[%s] VIB loaded version %s != required %s", host.Name(), loadedVersion, vspherelib.VibVersion)
			results = append(results, InstallationResult{Host: host.Name(), Success: false, Error: fmt.Errorf("loaded version %s != required %s", loadedVersion, vspherelib.VibVersion)})
		}
		fmt.Println()
	}

	printSummary(results)

	for _, result := range results {
		if !result.Success {
			os.Exit(1)
		}
	}
}

func buildSSHConfig(config *Config) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if config.SSHKeyFile != "" {
		privateKey, err := os.ReadFile(config.SSHKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if config.SSHPassword != "" {
		authMethods = append(authMethods, ssh.Password(config.SSHPassword))
		authMethods = append(authMethods, ssh.KeyboardInteractive(
			func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = config.SSHPassword
				}
				return answers, nil
			}))
	}

	return &ssh.ClientConfig{
		User:            "root",
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(config.SSHTimeout) * time.Second,
	}, nil
}

func validateConfig(config *Config) error {
	if config.RestartHostd {
		if config.SSHKeyFile == "" && config.SSHPassword == "" {
			return fmt.Errorf("SSH credentials required for hostd restart: provide --ssh-key-file or --ssh-password")
		}
		if config.SSHKeyFile != "" {
			if _, err := os.Stat(config.SSHKeyFile); os.IsNotExist(err) {
				return fmt.Errorf("SSH key file not found: %s", config.SSHKeyFile)
			}
		}
	}
	if _, err := os.Stat(config.VIBPath); os.IsNotExist(err) {
		return fmt.Errorf("VIB file not found: %s", config.VIBPath)
	}
	if config.VCenterUsername == "" || config.VCenterPassword == "" || config.VCenterHostname == "" {
		return fmt.Errorf("vCenter credentials are required (--vsphere-username, --vsphere-password, --vsphere-hostname)")
	}
	return nil
}

func discoverHosts(ctx context.Context, client vmware.Client, explicitHosts string) ([]*object.HostSystem, error) {
	allHosts, err := client.GetAllHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts from vCenter: %w", err)
	}

	if explicitHosts == "" {
		return allHosts, nil
	}

	// Filter to only the requested hosts by IP
	wantIPs := map[string]bool{}
	for _, h := range strings.Split(explicitHosts, ",") {
		trimmed := strings.TrimSpace(h)
		if trimmed != "" {
			wantIPs[trimmed] = true
		}
	}

	var filtered []*object.HostSystem
	for _, host := range allHosts {
		ip, err := vmware.GetHostIPAddress(ctx, host)
		if err != nil {
			logWarn("skipping host %s: failed to get IP address: %v", host.Reference().Value, err)
			continue
		}
		if wantIPs[ip] {
			filtered = append(filtered, host)
			delete(wantIPs, ip)
		}
	}

	if len(wantIPs) > 0 {
		remaining := make([]string, 0, len(wantIPs))
		for ip := range wantIPs {
			remaining = append(remaining, ip)
		}
		logWarn("Hosts not found in vCenter: %s", strings.Join(remaining, ", "))
	}

	return filtered, nil
}

func printSummary(results []InstallationResult) {
	fmt.Println("========================================")
	logInfo("Installation Summary")
	fmt.Println("========================================")

	successCount := 0
	failedHosts := make([]string, 0)

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failedHosts = append(failedHosts, result.Host)
		}
	}

	logInfo("Total hosts: %d", len(results))
	logInfo("Successful: %d", successCount)
	logInfo("Failed: %d", len(failedHosts))

	if len(failedHosts) > 0 {
		fmt.Println()
		logError("Failed hosts:")
		for _, host := range failedHosts {
			fmt.Printf("  - %s\n", host)
		}
	}
}

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
