package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ValidationError struct {
	Check   string
	Message string
	Hint    string
}

func (e *ValidationError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s\n  Hint: %s", e.Check, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Check, e.Message)
}

type Validator struct {
	executor PSExecutor
	rootPath string
}

func NewValidator(executor PSExecutor, rootPath string) *Validator {
	return &Validator{
		executor: executor,
		rootPath: rootPath,
	}
}

func (v *Validator) ValidateAll(ctx context.Context) error {
	checks := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Input validation", v.ValidateInput},
		{"HyperV availability", v.ValidateHyperVAvailable},
		{"HyperV permissions", v.ValidateHyperVPermissions},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return &ValidationError{
				Check:   "Pre-flight checks",
				Message: ctx.Err().Error(),
			}
		default:
			if err := check.fn(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *Validator) ValidateInput(ctx context.Context) error {
	if v.rootPath == "" {
		return nil
	}

	if strings.TrimSpace(v.rootPath) == "" {
		return &ValidationError{
			Check:   "Input validation",
			Message: "path cannot be empty or whitespace",
			Hint:    "Provide a valid directory path or omit the -path flag",
		}
	}

	if strings.ContainsAny(v.rootPath, "<>|\"") {
		return &ValidationError{
			Check:   "Input validation",
			Message: "path contains invalid characters",
			Hint:    "Remove characters: < > | \"",
		}
	}

	return nil
}

func (v *Validator) ValidateHyperVAvailable(ctx context.Context) error {
	// Check if Hyper-V PowerShell module is available
	cmd := `
		$ErrorActionPreference = 'Stop'
		try {
			$null = Get-Module -ListAvailable -Name Hyper-V
			if (-not (Get-Module -ListAvailable -Name Hyper-V)) {
				Write-Output "NOT_AVAILABLE"
			} else {
				Write-Output "AVAILABLE"
			}
		} catch {
			Write-Output "ERROR: $($_.Exception.Message)"
		}
	`

	out, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		// If PowerShell itself fails, we might be on Linux (testing)
		if psErr, ok := err.(*PSError); ok && psErr.IsCancelled() {
			return err
		}
		return &ValidationError{
			Check:   "HyperV availability",
			Message: "failed to check HyperV module",
			Hint:    "Ensure PowerShell is available and you're running on Windows",
		}
	}

	out = strings.TrimSpace(out)
	if out == "NOT_AVAILABLE" {
		return &ValidationError{
			Check:   "HyperV availability",
			Message: "Hyper-V PowerShell module is not installed",
			Hint:    "Install HyperV role: Install-WindowsFeature -Name Hyper-V -IncludeManagementTools",
		}
	}
	if strings.HasPrefix(out, "ERROR:") {
		return &ValidationError{
			Check:   "HyperV availability",
			Message: strings.TrimPrefix(out, "ERROR: "),
		}
	}

	return nil
}

func (v *Validator) ValidateHyperVPermissions(ctx context.Context) error {
	cmd := `
		$ErrorActionPreference = 'Stop'
		try {
			$null = Get-VM -ErrorAction Stop
			Write-Output "OK"
		} catch [Microsoft.HyperV.PowerShell.VirtualizationException] {
			Write-Output "NO_PERMISSION"
		} catch {
			Write-Output "ERROR: $($_.Exception.Message)"
		}
	`

	out, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		if psErr, ok := err.(*PSError); ok && psErr.IsCancelled() {
			return err
		}
		return &ValidationError{
			Check:   "HyperV permissions",
			Message: "failed to query HyperV",
			Hint:    "Run as Administrator or add user to 'Hyper-V Administrators' group",
		}
	}

	out = strings.TrimSpace(out)
	if out == "NO_PERMISSION" {
		return &ValidationError{
			Check:   "HyperV permissions",
			Message: "insufficient permissions to manage HyperV",
			Hint:    "Run as Administrator or add user to 'Hyper-V Administrators' group",
		}
	}
	if strings.HasPrefix(out, "ERROR:") {
		msg := strings.TrimPrefix(out, "ERROR: ")
		if strings.Contains(strings.ToLower(msg), "access") || strings.Contains(strings.ToLower(msg), "permission") {
			return &ValidationError{
				Check:   "HyperV permissions",
				Message: msg,
				Hint:    "Run as Administrator or add user to 'Hyper-V Administrators' group",
			}
		}
		return &ValidationError{
			Check:   "HyperV permissions",
			Message: msg,
		}
	}

	return nil
}

func (v *Validator) ValidateVMRequirements(ctx context.Context, vmName string, vmInfo map[string]interface{}) error {

	diskPaths := extractDiskPaths(vmInfo)
	if len(diskPaths) == 0 {
		return &ValidationError{
			Check:   fmt.Sprintf("VM '%s' requirements", vmName),
			Message: "VM has no attached hard disks",
			Hint:    "Attach at least one VHDX disk to the VM",
		}
	}

	for _, diskPath := range diskPaths {
		if err := v.validateDiskPath(ctx, diskPath); err != nil {
			return &ValidationError{
				Check:   fmt.Sprintf("VM '%s' disk access", vmName),
				Message: fmt.Sprintf("cannot access disk: %s", diskPath),
				Hint:    "Ensure the disk file exists and is readable",
			}
		}
	}

	if cpu, ok := vmInfo["ProcessorCount"].(float64); ok {
		if cpu < 1 {
			return &ValidationError{
				Check:   fmt.Sprintf("VM '%s' requirements", vmName),
				Message: "VM has invalid CPU count (< 1)",
			}
		}
	}

	if mem, ok := vmInfo["MemoryStartup"].(float64); ok {
		if mem < 1024*1024 { // Less than 1MB
			return &ValidationError{
				Check:   fmt.Sprintf("VM '%s' requirements", vmName),
				Message: "VM has invalid memory configuration (< 1MB)",
			}
		}
	}

	return nil
}

func (v *Validator) validateDiskPath(ctx context.Context, diskPath string) error {
	cmd := fmt.Sprintf(`
		$ErrorActionPreference = 'Stop'
		try {
			if (Test-Path -Path '%s' -PathType Leaf) {
				Write-Output "EXISTS"
			} else {
				Write-Output "NOT_FOUND"
			}
		} catch {
			Write-Output "ERROR: $($_.Exception.Message)"
		}
	`, diskPath)

	out, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		if psErr, ok := err.(*PSError); ok && psErr.IsCancelled() {
			return err
		}
		return fmt.Errorf("failed to check disk: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "NOT_FOUND" {
		return fmt.Errorf("disk file not found")
	}
	if strings.HasPrefix(out, "ERROR:") {
		return fmt.Errorf("%s", strings.TrimPrefix(out, "ERROR: "))
	}

	return nil
}

func (v *Validator) ValidateOutputDirectory(ctx context.Context, diskPath string) error {
	dir := filepath.Dir(diskPath)

	cmd := fmt.Sprintf(`
		$ErrorActionPreference = 'Stop'
		try {
			$testFile = Join-Path '%s' '.ovf-generator-test'
			[System.IO.File]::WriteAllText($testFile, 'test')
			Remove-Item $testFile -Force
			Write-Output "WRITABLE"
		} catch {
			Write-Output "NOT_WRITABLE: $($_.Exception.Message)"
		}
	`, dir)

	out, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		if psErr, ok := err.(*PSError); ok && psErr.IsCancelled() {
			return err
		}
		return &ValidationError{
			Check:   "Output directory",
			Message: fmt.Sprintf("cannot verify write access to %s", dir),
			Hint:    "Ensure you have write permissions to the directory containing the VHDX",
		}
	}

	out = strings.TrimSpace(out)
	if strings.HasPrefix(out, "NOT_WRITABLE:") {
		return &ValidationError{
			Check:   "Output directory",
			Message: fmt.Sprintf("cannot write to %s", dir),
			Hint:    "Ensure you have write permissions to the directory containing the VHDX",
		}
	}

	return nil
}

func (v *Validator) ValidateOVFNotExists(ctx context.Context, ovfPath string) (exists bool, err error) {
	_, err = os.Stat(ovfPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
