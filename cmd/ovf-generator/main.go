package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	hypervovf "github.com/kubev2v/forklift/pkg/hyperv-ovf"
)

const (
	toolName    = "ovf-generator"
	toolVersion = "1.0.0"
)

type Generator struct {
	executor  PSExecutor
	validator *Validator
	rootPath  string
}

func NewGenerator(executor PSExecutor, rootPath string) *Generator {
	return &Generator{
		executor:  executor,
		validator: NewValidator(executor, rootPath),
		rootPath:  rootPath,
	}
}

func main() {
	rootPath := flag.String("path", "", "Only process VMs with disks under this path")
	showHelp := flag.Bool("help", false, "Show help information")
	skipValidation := flag.Bool("skip-validation", false, "Skip pre-flight validation checks (not recommended)")

	// Custom usage function
	flag.Usage = func() {
		printHelp()
	}

	flag.Parse()

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)
		cancel()
	}()

	executor := &RealPSExecutor{}
	generator := NewGenerator(executor, *rootPath)

	exitCode := 0
	if err := generator.RunWithValidation(ctx, !*skipValidation); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "Operation cancelled by user")
			exitCode = 130
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func printHelp() {
	fmt.Printf(`%s v%s - HyperV OVF Metadata Generator

DESCRIPTION
  Generates OVF metadata files for HyperV VMs to enable migration to
  OpenShift Virtualization via Forklift.

  This tool runs locally on a Windows HyperV host and:
  1. Queries HyperV for VM metadata (CPU, RAM, disks, NICs)
  2. Detects guest OS via KVP Exchange (Integration Services)
  3. Generates OVF files next to VHDX disk files

USAGE
  %s [flags]

FLAGS
  -path <directory>     Only process VMs with disks under this path.
                        If not specified, processes all VMs.
  -skip-validation      Skip pre-flight validation checks (not recommended).
  -help                 Show this help information.

EXAMPLES
  Generate OVF for all VMs:
    %s

  Generate OVF only for VMs with disks in a specific folder:
    %s -path "C:\Hyper-V\Virtual Hard Disks"

PRE-FLIGHT CHECKS
  Before processing, the tool validates:
  - Input parameters (path format)
  - HyperV module availability
  - HyperV management permissions
  - VM disk accessibility
  - Output directory write permissions

OUTPUT
  For each VM, an OVF file is created in the same directory as the first VHDX:
    C:\VMs\vm1\
    ├── vm1.vhdx
    └── vm1.ovf      <- Generated

REQUIREMENTS
  - Windows Server with HyperV role
  - PowerShell (built-in)
  - Administrator privileges or 'Hyper-V Administrators' group membership
  - VMs can be running or stopped
    * Running VMs: Guest OS auto-detected via Integration Services
    * Stopped VMs: Guest OS defaults to "Unknown"

SIGNALS
  The tool handles SIGINT (Ctrl+C) and SIGTERM gracefully, stopping any
  in-progress operations without leaving partial files.

NOTES
  - VMs are NOT shut down; the tool only reads metadata
  - OVF files reference VHDX by filename (relative path)
  - All VHDX files for a VM must be in the same directory as the OVF

For more information, see: https://github.com/kubev2v/forklift
`, toolName, toolVersion, toolName, toolName, toolName)
}

func (g *Generator) RunWithValidation(ctx context.Context, validate bool) error {
	if validate {
		fmt.Println("Running pre-flight checks...")
		if err := g.validator.ValidateAll(ctx); err != nil {
			return fmt.Errorf("pre-flight check failed: %w", err)
		}
		fmt.Println("Pre-flight checks passed")
		fmt.Println()
	}

	return g.Run(ctx)
}

func (g *Generator) Run(ctx context.Context) error {
	fmt.Println("Querying local HyperV for VMs...")

	// 1. List all VMs
	vmNames, err := g.listVMs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list VMs: %w", err)
	}

	if len(vmNames) == 0 {
		fmt.Println("No VMs found.")
		return nil
	}

	fmt.Printf("Found %d VM(s)\n", len(vmNames))

	generated := 0
	skipped := 0
	failed := 0

	for _, vmName := range vmNames {
		select {
		case <-ctx.Done():
			fmt.Printf("\nCancelled. Generated %d OVF file(s) before interruption.\n", generated)
			return ctx.Err()
		default:
		}

		fmt.Printf("\nProcessing VM: %s\n", vmName)

		vmInfo, err := g.getVMInfo(ctx, vmName)
		if err != nil {
			fmt.Printf("Failed to get VM info: %v\n", err)
			failed++
			continue
		}

		diskPaths := extractDiskPaths(vmInfo)
		if len(diskPaths) == 0 {
			fmt.Printf("No disks found, skipping\n")
			skipped++
			continue
		}

		fmt.Printf("  Disks: %s\n", strings.Join(diskPaths, ", "))

		if g.rootPath != "" {
			absRoot, _ := filepath.Abs(g.rootPath)
			match := false
			for _, dp := range diskPaths {
				if strings.HasPrefix(strings.ToLower(dp), strings.ToLower(absRoot)) {
					match = true
					break
				}
			}
			if !match {
				fmt.Printf("Disks not under %s, skipping\n", g.rootPath)
				skipped++
				continue
			}
		}

		if err := g.validator.ValidateVMRequirements(ctx, vmName, vmInfo); err != nil {
			fmt.Printf("Validation failed: %v\n", err)
			failed++
			continue
		}

		if err := g.validator.ValidateOutputDirectory(ctx, diskPaths[0]); err != nil {
			fmt.Printf("Cannot write OVF: %v\n", err)
			failed++
			continue
		}

		ovfPath := hypervovf.RemoveFileExtension(diskPaths[0]) + ".ovf"
		if exists, _ := g.validator.ValidateOVFNotExists(ctx, ovfPath); exists {
			fmt.Printf("OVF already exists, will overwrite: %s\n", ovfPath)
		}

		guestOS := g.getGuestOSInfo(ctx, vmName)
		vmInfo["GuestOSInfo"] = guestOS

		if err := hypervovf.FormatFromHyperV(vmInfo, diskPaths); err != nil {
			fmt.Printf("Failed to generate OVF: %v\n", err)
			failed++
			continue
		}

		fmt.Printf("OVF generated successfully\n")
		generated++
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Summary: %d generated, %d skipped, %d failed\n", generated, skipped, failed)
	fmt.Println("═══════════════════════════════════════")

	if failed > 0 {
		return fmt.Errorf("%d VM(s) failed to process", failed)
	}

	return nil
}

func (g *Generator) listVMs(ctx context.Context) ([]string, error) {
	out, err := g.executor.Execute(ctx, "Get-VM | Select-Object -ExpandProperty Name")
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

func (g *Generator) getVMInfo(ctx context.Context, vmName string) (map[string]interface{}, error) {
	cmd := fmt.Sprintf(`
		$ErrorActionPreference = 'Stop'
		$vm = Get-VM -Name '%s'
		$disks = Get-VMHardDiskDrive -VMName '%s' | Select-Object -Property Path, ControllerType, ControllerNumber, ControllerLocation
		$nics = Get-VMNetworkAdapter -VMName '%s' | Select-Object -Property Name
		
		@{
			Name = $vm.Name
			Generation = $vm.Generation
			ProcessorCount = $vm.ProcessorCount
			MemoryStartup = $vm.MemoryStartup
			HardDrives = @($disks)
			NetworkAdapters = @($nics)
		} | ConvertTo-Json -Depth 3
	`, vmName, vmName, vmName)

	out, err := g.executor.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nRaw: %s", err, out)
	}
	return result, nil
}

func (g *Generator) getGuestOSInfo(ctx context.Context, vmName string) map[string]interface{} {
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

	out, err := g.executor.Execute(ctx, cmd)
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
