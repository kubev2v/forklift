package powershell

import (
	"strings"
	"testing"
)

// buildPathList converts a Go string slice into a PowerShell array literal
// suitable for injection into TestPaths. Each element is single-quoted with
// internal single quotes doubled for escaping.
func buildPathList(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	escaped := make([]string, len(paths))
	for i, p := range paths {
		escaped[i] = "'" + strings.ReplaceAll(p, "'", "''") + "'"
	}
	return strings.Join(escaped, ",")
}

func TestBuildCommand_EscapesSingleQuotes(t *testing.T) {
	result := BuildCommand("Get-VM -Name '%s'", "it's a test")
	expected := "Get-VM -Name 'it''s a test'"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func Test_buildPathList(t *testing.T) {
	tests := []struct {
		name   string
		paths  []string
		expect string
	}{
		{
			name:   "empty",
			paths:  nil,
			expect: "",
		},
		{
			name:   "single path",
			paths:  []string{`C:\VMs\disk.vhdx`},
			expect: `'C:\VMs\disk.vhdx'`,
		},
		{
			name:   "multiple paths",
			paths:  []string{`C:\a.vhdx`, `C:\b.vhdx`},
			expect: `'C:\a.vhdx','C:\b.vhdx'`,
		},
		{
			name:   "path with single quote",
			paths:  []string{`C:\VM's\disk.vhdx`},
			expect: `'C:\VM''s\disk.vhdx'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPathList(tt.paths)
			if got != tt.expect {
				t.Errorf("buildPathList(%v) = %q, want %q", tt.paths, got, tt.expect)
			}
		})
	}
}

func TestDiffDiskPath(t *testing.T) {
	got := DiffDiskPath("forklift-abc123", 0)
	expected := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	if got != expected {
		t.Errorf("DiffDiskPath = %q, want %q", got, expected)
	}

	got = DiffDiskPath("forklift-abc123", 2)
	expected = `C:\iscsi-targets\forklift-abc123-disk2.vhdx`
	if got != expected {
		t.Errorf("DiffDiskPath = %q, want %q", got, expected)
	}
}

func TestDiffDiskPattern(t *testing.T) {
	got := DiffDiskPattern("forklift-abc123")
	expected := `C:\iscsi-targets\forklift-abc123-*`
	if got != expected {
		t.Errorf("DiffDiskPattern = %q, want %q", got, expected)
	}
}

func TestCreateIscsiVirtualDisk_Template(t *testing.T) {
	cmd := BuildCommand(CreateIscsiVirtualDisk,
		`C:\iscsi-targets\forklift-abc123-disk0.vhdx`,
		`C:\VMs\win2019\disk0.vhdx`)

	if !strings.Contains(cmd, "-ParentPath") {
		t.Error("CreateIscsiVirtualDisk must use -ParentPath, not -Path")
	}
	if !strings.Contains(cmd, "New-IscsiVirtualDisk") {
		t.Error("expected New-IscsiVirtualDisk cmdlet")
	}
	if !strings.Contains(cmd, `C:\VMs\win2019\disk0.vhdx`) {
		t.Error("expected parent VHDX path in output")
	}
}

func TestAddIscsiVirtualDiskTargetMapping_Template(t *testing.T) {
	cmd := BuildCommand(AddIscsiVirtualDiskTargetMapping,
		"forklift-abc123",
		`C:\iscsi-targets\forklift-abc123-disk0.vhdx`,
		"0")

	if !strings.Contains(cmd, "Add-IscsiVirtualDiskTargetMapping") {
		t.Error("expected Add-IscsiVirtualDiskTargetMapping cmdlet")
	}
	if !strings.Contains(cmd, "$lun=0") {
		t.Error("expected LUN assignment in command")
	}
	if !strings.Contains(cmd, "-Lun $lun") {
		t.Error("expected -Lun parameter reference in command")
	}
}

func TestRemoveIscsiVirtualDisk_Template(t *testing.T) {
	cmd := BuildCommand(RemoveIscsiVirtualDisk,
		`C:\iscsi-targets\forklift-abc123-disk0.vhdx`)

	if !strings.Contains(cmd, "Remove-IscsiVirtualDisk") {
		t.Error("expected Remove-IscsiVirtualDisk cmdlet")
	}
	if !strings.Contains(cmd, "Remove-Item") {
		t.Error("expected Remove-Item for filesystem cleanup")
	}
}

func TestCleanupIscsiDiffDisks_Template(t *testing.T) {
	cmd := BuildCommand(CleanupIscsiDiffDisks,
		"forklift-abc123",
		`C:\iscsi-targets\forklift-abc123-*`)

	if !strings.Contains(cmd, "Get-IscsiServerTarget") {
		t.Error("expected target query for LUN mappings")
	}
	if !strings.Contains(cmd, "Remove-IscsiVirtualDiskTargetMapping") {
		t.Error("expected mapping removal")
	}
	if !strings.Contains(cmd, "Get-ChildItem") {
		t.Error("expected filesystem cleanup via Get-ChildItem")
	}
}

func TestTestPath_Template(t *testing.T) {
	cmd := BuildCommand(TestPath, `C:\VMs\disk.vhdx`)
	expected := `Test-Path -Path 'C:\VMs\disk.vhdx' -PathType Leaf`
	if cmd != expected {
		t.Errorf("TestPath = %q, want %q", cmd, expected)
	}
}

func TestTestPaths_Template(t *testing.T) {
	pathList := buildPathList([]string{`C:\a.vhdx`, `C:\b.vhdx`})
	cmd := BuildCommand(TestPaths, pathList)

	if !strings.Contains(cmd, `'C:\a.vhdx'`) {
		t.Error("expected first path in command")
	}
	if !strings.Contains(cmd, `'C:\b.vhdx'`) {
		t.Error("expected second path in command")
	}
	if !strings.Contains(cmd, "Missing") {
		t.Error("expected Missing key in JSON output template")
	}
}

func TestEnsureIscsiTargetDir_Template(t *testing.T) {
	cmd := BuildCommand(EnsureIscsiTargetDir, IscsiTargetDir, IscsiTargetDir)
	if !strings.Contains(cmd, "New-Item") {
		t.Error("expected New-Item for directory creation")
	}
	if !strings.Contains(cmd, "Test-Path") {
		t.Error("expected Test-Path for idempotency check")
	}
}
