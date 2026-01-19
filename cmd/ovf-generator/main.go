// Standalone OVF Generator for HyperV
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	hypervovf "github.com/kubev2v/forklift/pkg/hyperv-ovf"
)

func main() {
	// Parse flags
	rootPath := flag.String("path", "", "Optional: only process VMs with disks under this path")
	flag.Parse()

	fmt.Println("Querying local HyperV for VMs...")

	// 1. List all VMs
	vmNames, err := listVMs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list VMs: %v\n", err)
		os.Exit(1)
	}

	if len(vmNames) == 0 {
		fmt.Println("No VMs found.")
		return
	}

	fmt.Printf("Found %d VM(s)\n", len(vmNames))

	// 2. Process each VM
	generated := 0
	for _, vmName := range vmNames {
		fmt.Printf("\nProcessing VM: %s\n", vmName)

		// Get VM info
		vmInfo, err := getVMInfo(vmName)
		if err != nil {
			fmt.Printf("  Failed to get VM info: %v\n", err)
			continue
		}

		// Extract disk paths
		diskPaths := extractDiskPaths(vmInfo)
		if len(diskPaths) == 0 {
			fmt.Printf("  No disks found, skipping\n")
			continue
		}

		fmt.Printf("  Disks: %s\n", strings.Join(diskPaths, ", "))

		// Filter by path if specified
		if *rootPath != "" {
			absRoot, _ := filepath.Abs(*rootPath)
			match := false
			for _, dp := range diskPaths {
				if strings.HasPrefix(strings.ToLower(dp), strings.ToLower(absRoot)) {
					match = true
					break
				}
			}
			if !match {
				fmt.Printf("  Disks not under %s, skipping\n", *rootPath)
				continue
			}
		}

		// Try to get guest OS info (VM must be running with integration services)
		guestOS := getGuestOSInfo(vmName)
		vmInfo["GuestOSInfo"] = guestOS

		// Generate OVF (in same folder as first disk)
		if err := hypervovf.FormatFromHyperV(vmInfo, diskPaths); err != nil {
			fmt.Printf("  Failed to generate OVF: %v\n", err)
			continue
		}

		generated++
	}

	fmt.Printf("\n Generated %d OVF file(s)\n", generated)
}

// listVMs returns all VM names from local HyperV
func listVMs() ([]string, error) {
	out, err := runPS("Get-VM | Select-Object -ExpandProperty Name")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var names []string
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// getVMInfo returns VM details as map
func getVMInfo(vmName string) (map[string]interface{}, error) {
	cmd := fmt.Sprintf(`
		$vm = Get-VM -Name '%s'
		$disks = Get-VMHardDiskDrive -VMName '%s' | Select-Object -Property Path
		$nics = Get-VMNetworkAdapter -VMName '%s' | Select-Object -Property Name
		
		@{
			Name = $vm.Name
			ProcessorCount = $vm.ProcessorCount
			MemoryStartup = $vm.MemoryStartup
			HardDrives = @($disks)
			NetworkAdapters = @($nics)
		} | ConvertTo-Json -Depth 3
	`, vmName, vmName, vmName)

	out, err := runPS(cmd)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nRaw: %s", err, out)
	}
	return result, nil
}

// getGuestOSInfo tries to get OS info from running VM via KVP exchange
// Returns defaults if VM is off or integration services unavailable
func getGuestOSInfo(vmName string) map[string]interface{} {
	cmd := fmt.Sprintf(`
		$ErrorActionPreference = 'SilentlyContinue'
		$vm = Get-WmiObject -Namespace root\virtualization\v2 -Class Msvm_ComputerSystem -Filter "ElementName='%s'"
		if ($vm) {
			$kvp = $vm.GetRelated('Msvm_KvpExchangeComponent')
			if ($kvp -and $kvp.GuestIntrinsicExchangeItems) {
				$osName = ''
				$osVersion = ''
				foreach ($item in $kvp.GuestIntrinsicExchangeItems) {
					$xml = [xml]$item
					$name = $xml.INSTANCE.PROPERTY | Where-Object { $_.NAME -eq 'Name' } | Select-Object -ExpandProperty VALUE
					$value = $xml.INSTANCE.PROPERTY | Where-Object { $_.NAME -eq 'Data' } | Select-Object -ExpandProperty VALUE
					if ($name -eq 'OSName') { $osName = $value }
					if ($name -eq 'OSVersion') { $osVersion = $value }
				}
				if ($osName) {
					@{
						Caption = $osName
						Version = $osVersion
						OSArchitecture = '64-bit'
					} | ConvertTo-Json
				} else {
					$null
				}
			} else {
				$null
			}
		} else {
			$null
		}
	`, vmName)

	out, err := runPS(cmd)
	if err != nil || strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "null" {
		return map[string]interface{}{
			"Caption":        "Unknown",
			"Version":        "",
			"OSArchitecture": "64-bit",
		}
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return map[string]interface{}{
			"Caption":        "Unknown",
			"Version":        "",
			"OSArchitecture": "64-bit",
		}
	}
	return result
}

// extractDiskPaths extracts VHDX paths from VM info
func extractDiskPaths(vmInfo map[string]interface{}) []string {
	var paths []string

	drives, ok := vmInfo["HardDrives"]
	if !ok {
		return paths
	}

	switch v := drives.(type) {
	case []interface{}:
		for _, drive := range v {
			if d, ok := drive.(map[string]interface{}); ok {
				if path, ok := d["Path"].(string); ok && path != "" {
					paths = append(paths, path)
				}
			}
		}
	case map[string]interface{}:
		if path, ok := v["Path"].(string); ok && path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// runPS executes PowerShell command locally and returns output
func runPS(command string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("powershell error: %s\nstderr: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(out), nil
}
