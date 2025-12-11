package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload"
	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

const (
	defaultVIBPath = "/bin/vmkfstools-wrapper.vib"
	colorReset     = "\033[0m"
	colorRed       = "\033[31m"
	colorGreen     = "\033[32m"
	colorYellow    = "\033[33m"
)

type Config struct {
	// VIB installation
	VIBPath   string
	ESXiHosts string // comma-separated list

	// vSphere connection
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
	flag.StringVar(&config.ESXiHosts, "esxi-hosts", getEnv("ESXI_HOSTS", ""), "Comma-separated list of ESXi hosts")
	flag.StringVar(&config.VCenterUsername, "vsphere-username", os.Getenv("GOVMOMI_USERNAME"), "vSphere username")
	flag.StringVar(&config.VCenterPassword, "vsphere-password", os.Getenv("GOVMOMI_PASSWORD"), "vSphere password")
	flag.StringVar(&config.VCenterHostname, "vsphere-hostname", os.Getenv("GOVMOMI_HOSTNAME"), "vSphere hostname")
	flag.BoolVar(&config.VCenterInsecure, "vsphere-insecure", getEnvBool("GOVMOMI_INSECURE", false), "Skip SSL verification for vSphere")
	flag.StringVar(&config.Datacenter, "datacenter", "", "Datacenter name to filter ESXi hosts (optional)")

	klog.InitFlags(nil)
	flag.Parse()

	if err := validateConfig(config); err != nil {
		logError("Configuration error: %v", err)
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	vcenterURL := fmt.Sprintf("https://%s/sdk", config.VCenterHostname)
	client, err := vmware.NewClient(vcenterURL, config.VCenterUsername, config.VCenterPassword)
	if err != nil {
		logError("Failed to create vSphere client: %v", err)
		os.Exit(1)
	}

	// Get list of ESXi hosts
	hosts, err := getESXiHosts(ctx, client, config)
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

	// Install VIB on each host using API
	results := make([]InstallationResult, 0, len(hosts))
	for _, host := range hosts {
		result := installVIBOnHost(ctx, client, config, host)
		results = append(results, result)
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
	if _, err := os.Stat(config.VIBPath); os.IsNotExist(err) {
		return fmt.Errorf("VIB file not found: %s", config.VIBPath)
	}

	if config.VCenterUsername == "" || config.VCenterPassword == "" || config.VCenterHostname == "" {
		return fmt.Errorf("vCenter credentials are required (--vsphere-username, --vsphere-password, --vsphere-hostname)")
	}

	return nil
}

func getESXiHosts(ctx context.Context, client vmware.Client, config *Config) ([]*object.HostSystem, error) {
	// If hosts are explicitly provided, get all hosts and filter by name
	allHosts, err := client.GetAllHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts: %w", err)
	}
	if config.ESXiHosts != "" {
		requestedHosts := strings.Split(config.ESXiHosts, ",")
		requestedHostsMap := make(map[string]bool)
		for _, host := range requestedHosts {
			trimmed := strings.TrimSpace(host)
			if trimmed != "" {
				requestedHostsMap[trimmed] = true
			}
		}

		var filteredHosts []*object.HostSystem
		for _, host := range allHosts {
			if requestedHostsMap[host.Name()] {
				filteredHosts = append(filteredHosts, host)
				logInfo("  Found requested host: %s", host.Name())
			}
		}
		return filteredHosts, nil
	}

	// Otherwise, get all hosts from vCenter
	logInfo("Discovering all ESXi hosts from vCenter...")
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}

	for _, host := range allHosts {
		logInfo("  Discovered: %s", host.Name())
	}

	return allHosts, nil
}

func installVIBOnHost(ctx context.Context, client vmware.Client, config *Config, host *object.HostSystem) InstallationResult {
	hostName := host.Name()
	logInfo("[%s] Starting VIB installation using vSphere API...", hostName)

	err := vsphere_offload.EnsureVib(ctx, client, host, config.VIBPath)
	if err != nil {
		logError("[%s] VIB installation failed: %v", hostName, err)
		return InstallationResult{Host: hostName, Success: false, Error: err}
	}

	logSuccess("[%s] ✓ VIB installation successful!", hostName)
	return InstallationResult{Host: hostName, Success: true, Error: nil}
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
