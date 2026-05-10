package iscsi

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
)

// mockDriver implements driver.HyperVDriver for unit tests.
// Commands and errors are keyed by the exact PowerShell command string.
// Hooks run after a command executes, allowing state transitions (e.g. making
// a target "disappear" after the remove command fires).
type mockDriver struct {
	commands map[string]string
	errors   map[string]error
	called   []string // records every command executed, in order
	hooks    map[string]func(m *mockDriver)
}

func newMockDriver() *mockDriver {
	return &mockDriver{
		commands: make(map[string]string),
		errors:   make(map[string]error),
		hooks:    make(map[string]func(m *mockDriver)),
	}
}

func (m *mockDriver) Connect() error         { return nil }
func (m *mockDriver) Close() error           { return nil }
func (m *mockDriver) IsAlive() (bool, error) { return true, nil }
func (m *mockDriver) ListAllDomains() ([]driver.Domain, error) {
	return nil, errors.New("not implemented in mock")
}
func (m *mockDriver) LookupDomainByName(string) (driver.Domain, error) {
	return nil, errors.New("not implemented in mock")
}
func (m *mockDriver) LookupDomainByUUIDString(string) (driver.Domain, error) {
	return nil, errors.New("not implemented in mock")
}
func (m *mockDriver) ListAllNetworks() ([]driver.Network, error) {
	return nil, errors.New("not implemented in mock")
}
func (m *mockDriver) LookupNetworkByUUIDString(string) (driver.Network, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockDriver) ExecuteCommand(command string) (string, error) {
	m.called = append(m.called, command)
	if e, ok := m.errors[command]; ok {
		return "", e
	}
	out, ok := m.commands[command]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", command)
	}
	if h, ok := m.hooks[command]; ok {
		h(m)
	}
	return out, nil
}

func TestCheckReadiness_AllReady(t *testing.T) {
	drv := newMockDriver()
	drv.commands[ps.CheckIscsiTargetFeature] = `{"Installed":true}`
	drv.commands[ps.CheckIscsiFirewallPort] = `{"Open":true}`

	c := NewTargetClient(drv)
	r, err := c.CheckReadiness()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.FeatureInstalled {
		t.Error("expected FeatureInstalled=true")
	}
	if !r.FirewallOpen {
		t.Error("expected FirewallOpen=true")
	}
	if !r.Ready() {
		t.Error("expected Ready()=true")
	}
}

func TestCheckReadiness_FeatureNotInstalled(t *testing.T) {
	drv := newMockDriver()
	drv.commands[ps.CheckIscsiTargetFeature] = `{"Installed":false}`

	c := NewTargetClient(drv)
	r, err := c.CheckReadiness()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.FeatureInstalled {
		t.Error("expected FeatureInstalled=false")
	}
	if r.FirewallOpen {
		t.Error("firewall should not be checked when feature is missing")
	}
	if r.Ready() {
		t.Error("expected Ready()=false")
	}
}

func TestCheckReadiness_PortClosed(t *testing.T) {
	drv := newMockDriver()
	drv.commands[ps.CheckIscsiTargetFeature] = `{"Installed":true}`
	drv.commands[ps.CheckIscsiFirewallPort] = `{"Open":false}`

	c := NewTargetClient(drv)
	r, err := c.CheckReadiness()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.FeatureInstalled {
		t.Error("expected FeatureInstalled=true")
	}
	if r.FirewallOpen {
		t.Error("expected FirewallOpen=false")
	}
	if r.Ready() {
		t.Error("expected Ready()=false")
	}
}

func TestCheckReadiness_FeatureCheckFails(t *testing.T) {
	drv := newMockDriver()
	drv.errors[ps.CheckIscsiTargetFeature] = errors.New("WinRM timeout")

	c := NewTargetClient(drv)
	r, err := c.CheckReadiness()
	if err == nil {
		t.Fatal("expected error on WinRM failure, got nil")
	}
	if r.FeatureInstalled {
		t.Error("expected FeatureInstalled=false on failure")
	}
}

func TestCheckReadiness_PortCheckFails(t *testing.T) {
	drv := newMockDriver()
	drv.commands[ps.CheckIscsiTargetFeature] = `{"Installed":true}`
	drv.errors[ps.CheckIscsiFirewallPort] = errors.New("WinRM timeout")

	c := NewTargetClient(drv)
	r, err := c.CheckReadiness()
	if err == nil {
		t.Fatal("expected error on WinRM failure, got nil")
	}
	if !r.FeatureInstalled {
		t.Error("expected FeatureInstalled=true")
	}
	if r.FirewallOpen {
		t.Error("expected FirewallOpen=false on failure")
	}
}

func TestCreateTarget_Success(t *testing.T) {
	targetName := "forklift-abc123"
	iqn := "iqn.2099-01.io.forklift:copy-test-migration"
	cmd := ps.BuildCommand(ps.CreateIscsiTarget, targetName, iqn)

	drv := newMockDriver()
	drv.commands[cmd] = `{"TargetIqn":"iqn.1991-05.com.microsoft:win-host-forklift-abc123-target","Created":true}`

	c := NewTargetClient(drv)
	res, err := c.CreateTarget(targetName, iqn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Created {
		t.Error("expected Created=true")
	}
	if res.TargetIQN == "" {
		t.Error("expected non-empty TargetIQN")
	}
}

func TestCreateTarget_AlreadyExists(t *testing.T) {
	targetName := "forklift-abc123"
	iqn := "iqn.2099-01.io.forklift:copy-test-migration"
	cmd := ps.BuildCommand(ps.CreateIscsiTarget, targetName, iqn)

	drv := newMockDriver()
	drv.commands[cmd] = `{"TargetIqn":"iqn.1991-05.com.microsoft:win-host-forklift-abc123-target","Created":false}`

	c := NewTargetClient(drv)
	res, err := c.CreateTarget(targetName, iqn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Created {
		t.Error("expected Created=false for existing target")
	}
}

func TestCreateTarget_CommandFails(t *testing.T) {
	targetName := "forklift-abc123"
	iqn := "iqn.2099-01.io.forklift:copy-test-migration"
	cmd := ps.BuildCommand(ps.CreateIscsiTarget, targetName, iqn)

	drv := newMockDriver()
	drv.errors[cmd] = errors.New("feature not installed")

	c := NewTargetClient(drv)
	_, err := c.CreateTarget(targetName, iqn)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateTarget_EmptyResponse(t *testing.T) {
	targetName := "forklift-abc123"
	iqn := "iqn.2099-01.io.forklift:copy-test-migration"
	cmd := ps.BuildCommand(ps.CreateIscsiTarget, targetName, iqn)

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	_, err := c.CreateTarget(targetName, iqn)
	if err == nil {
		t.Fatal("expected error on empty response")
	}
}

func TestCreateTarget_InvalidJSON(t *testing.T) {
	targetName := "forklift-abc123"
	iqn := "iqn.2099-01.io.forklift:copy-test-migration"
	cmd := ps.BuildCommand(ps.CreateIscsiTarget, targetName, iqn)

	drv := newMockDriver()
	drv.commands[cmd] = "not valid json"

	c := NewTargetClient(drv)
	_, err := c.CreateTarget(targetName, iqn)
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestGetTarget_Found(t *testing.T) {
	targetName := "forklift-abc123"
	cmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)

	drv := newMockDriver()
	drv.commands[cmd] = `{"TargetIqn":"iqn.1991-05.com.microsoft:win-host-forklift-abc123-target","Status":"Connected","LunCount":2}`

	c := NewTargetClient(drv)
	info, err := c.GetTarget(targetName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil TargetInfo")
	}
	if info.LunCount != 2 {
		t.Errorf("expected LunCount=2, got %d", info.LunCount)
	}
	if info.Status != "Connected" {
		t.Errorf("expected Status=Connected, got %s", info.Status)
	}
}

func TestGetTarget_NotFound(t *testing.T) {
	targetName := "forklift-nonexistent"
	cmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	_, err := c.GetTarget(targetName)
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("expected ErrTargetNotFound, got: %v", err)
	}
}

func TestGetTarget_CommandFails(t *testing.T) {
	targetName := "forklift-abc123"
	cmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)

	drv := newMockDriver()
	drv.errors[cmd] = errors.New("WinRM error")

	c := NewTargetClient(drv)
	_, err := c.GetTarget(targetName)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveTarget_Success(t *testing.T) {
	targetName := "forklift-abc123"
	getCmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)
	removeCmd := ps.BuildCommand(ps.RemoveIscsiServerTarget, targetName)

	drv := newMockDriver()
	drv.commands[getCmd] = `{"TargetIqn":"iqn.test","Status":"Connected","LunCount":0}`
	drv.commands[removeCmd] = ""
	// After remove executes, GetTarget should return empty (target gone).
	drv.hooks[removeCmd] = func(m *mockDriver) {
		m.commands[getCmd] = ""
	}

	c := NewTargetClient(drv)
	if err := c.RemoveTarget(targetName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTarget_NotFound(t *testing.T) {
	targetName := "forklift-abc123"
	getCmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)

	drv := newMockDriver()
	drv.commands[getCmd] = ""

	c := NewTargetClient(drv)
	if err := c.RemoveTarget(targetName); err != nil {
		t.Fatalf("expected no error for non-existent target, got: %v", err)
	}
}

func TestEnsureTargetDir_Success(t *testing.T) {
	cmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	if err := c.EnsureTargetDir(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureTargetDir_Fails(t *testing.T) {
	cmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)

	drv := newMockDriver()
	drv.errors[cmd] = errors.New("access denied")

	c := NewTargetClient(drv)
	if err := c.EnsureTargetDir(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateVirtualDisk_NewDisk(t *testing.T) {
	diffPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	parentPath := `C:\VMs\win2019\disk0.vhdx`
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffPath)
	createCmd := ps.BuildCommand(ps.NewIscsiVirtualDisk, diffPath, parentPath)

	drv := newMockDriver()
	drv.commands[checkCmd] = ""
	drv.commands[createCmd] = fmt.Sprintf(`{"Path":"%s"}`, strings.ReplaceAll(diffPath, `\`, `\\`))

	c := NewTargetClient(drv)
	res, err := c.CreateVirtualDisk(diffPath, parentPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DevicePath == "" {
		t.Error("expected non-empty DevicePath")
	}
}

func TestCreateVirtualDisk_ExistingWithSameParent(t *testing.T) {
	diffPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	parentPath := `C:\VMs\win2019\disk0.vhdx`
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffPath)
	parentCmd := ps.BuildCommand(ps.GetVHDParentPath, diffPath)

	drv := newMockDriver()
	drv.commands[checkCmd] = fmt.Sprintf(`{"Path":"%s"}`, strings.ReplaceAll(diffPath, `\`, `\\`))
	drv.commands[parentCmd] = parentPath

	c := NewTargetClient(drv)
	res, err := c.CreateVirtualDisk(diffPath, parentPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DevicePath == "" {
		t.Error("expected non-empty DevicePath for reused disk")
	}
}

func TestCreateVirtualDisk_CreateFails(t *testing.T) {
	diffPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	parentPath := `C:\VMs\win2019\disk0.vhdx`
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffPath)
	createCmd := ps.BuildCommand(ps.NewIscsiVirtualDisk, diffPath, parentPath)

	drv := newMockDriver()
	drv.commands[checkCmd] = ""
	drv.errors[createCmd] = errors.New("disk creation failed")

	c := NewTargetClient(drv)
	_, err := c.CreateVirtualDisk(diffPath, parentPath)
	if err == nil {
		t.Fatal("expected error on create failure")
	}
}

func TestMapDiskToTarget_Success(t *testing.T) {
	targetName := "forklift-abc123"
	diskPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	cmd := ps.BuildCommand(ps.AddIscsiVirtualDiskTargetMapping, targetName, diskPath, "0")

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	if err := c.MapDiskToTarget(targetName, diskPath, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMapDiskToTarget_Fails(t *testing.T) {
	targetName := "forklift-abc123"
	diskPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	cmd := ps.BuildCommand(ps.AddIscsiVirtualDiskTargetMapping, targetName, diskPath, "0")

	drv := newMockDriver()
	drv.errors[cmd] = errors.New("target not found")

	c := NewTargetClient(drv)
	if err := c.MapDiskToTarget(targetName, diskPath, 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestUnmapDiskFromTarget_Success(t *testing.T) {
	targetName := "forklift-abc123"
	diskPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	cmd := ps.BuildCommand(ps.RemoveIscsiVirtualDiskTargetMapping, targetName, diskPath)

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	if err := c.UnmapDiskFromTarget(targetName, diskPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveVirtualDisk_Success(t *testing.T) {
	diskPath := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	cmd := ps.BuildCommand(ps.RemoveIscsiVirtualDisk, diskPath)

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	if err := c.RemoveVirtualDisk(diskPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupDiffDisks_Success(t *testing.T) {
	targetName := "forklift-abc123"
	pattern := ps.DiffDiskPattern(targetName)
	disk0 := `C:\iscsi-targets\forklift-abc123-disk0.vhdx`
	disk1 := `C:\iscsi-targets\forklift-abc123-disk1.vhdx`

	listCmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)
	unmap0 := ps.BuildCommand(ps.RemoveIscsiVirtualDiskTargetMapping, targetName, disk0)
	unmap1 := ps.BuildCommand(ps.RemoveIscsiVirtualDiskTargetMapping, targetName, disk1)
	rmDisk0 := ps.BuildCommand(ps.RemoveIscsiVirtualDisk, disk0)
	rmDisk1 := ps.BuildCommand(ps.RemoveIscsiVirtualDisk, disk1)
	rmFiles := ps.BuildCommand(ps.RemoveFilesByPattern, pattern)

	drv := newMockDriver()
	drv.commands[listCmd] = fmt.Sprintf(`[{"Path":"%s","Lun":0},{"Path":"%s","Lun":1}]`,
		strings.ReplaceAll(disk0, `\`, `\\`), strings.ReplaceAll(disk1, `\`, `\\`))
	drv.commands[unmap0] = ""
	drv.commands[unmap1] = ""
	drv.commands[rmDisk0] = ""
	drv.commands[rmDisk1] = ""
	drv.commands[rmFiles] = ""

	c := NewTargetClient(drv)
	if err := c.CleanupDiffDisks(targetName, pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupDiffDisks_NoMappings(t *testing.T) {
	targetName := "forklift-abc123"
	pattern := ps.DiffDiskPattern(targetName)

	listCmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)
	rmFiles := ps.BuildCommand(ps.RemoveFilesByPattern, pattern)

	drv := newMockDriver()
	drv.commands[listCmd] = ""
	drv.commands[rmFiles] = ""

	c := NewTargetClient(drv)
	if err := c.CleanupDiffDisks(targetName, pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListLunMappings_Multiple(t *testing.T) {
	targetName := "forklift-abc123"
	cmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)

	drv := newMockDriver()
	drv.commands[cmd] = `[{"Path":"C:\\iscsi-targets\\forklift-abc123-disk0.vhdx","Lun":0},{"Path":"C:\\iscsi-targets\\forklift-abc123-disk1.vhdx","Lun":1}]`

	c := NewTargetClient(drv)
	mappings, err := c.ListLunMappings(targetName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].Lun != 0 || mappings[1].Lun != 1 {
		t.Error("unexpected LUN values")
	}
}

func TestListLunMappings_Single(t *testing.T) {
	targetName := "forklift-abc123"
	cmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)

	drv := newMockDriver()
	drv.commands[cmd] = `{"Path":"C:\\iscsi-targets\\forklift-abc123-disk0.vhdx","Lun":0}`

	c := NewTargetClient(drv)
	mappings, err := c.ListLunMappings(targetName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
}

func TestListLunMappings_Empty(t *testing.T) {
	targetName := "forklift-abc123"
	cmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)

	drv := newMockDriver()
	drv.commands[cmd] = ""

	c := NewTargetClient(drv)
	mappings, err := c.ListLunMappings(targetName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mappings) != 0 {
		t.Errorf("expected 0 mappings, got %d", len(mappings))
	}
}

func TestListLunMappings_InvalidJSON(t *testing.T) {
	targetName := "forklift-abc123"
	cmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)

	drv := newMockDriver()
	drv.commands[cmd] = "bad json"

	c := NewTargetClient(drv)
	_, err := c.ListLunMappings(targetName)
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestSetupDiskForMigration_Success(t *testing.T) {
	targetName := "forklift-abc123"
	parentPath := `C:\VMs\win2019\disk0.vhdx`
	diskIndex := 0
	diffPath := ps.DiffDiskPath(targetName, diskIndex)

	ensureCmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffPath)
	createCmd := ps.BuildCommand(ps.NewIscsiVirtualDisk, diffPath, parentPath)
	mapCmd := ps.BuildCommand(ps.AddIscsiVirtualDiskTargetMapping, targetName, diffPath, "0")

	drv := newMockDriver()
	drv.commands[ensureCmd] = ""
	drv.commands[checkCmd] = ""
	drv.commands[createCmd] = fmt.Sprintf(`{"Path":"%s"}`, strings.ReplaceAll(diffPath, `\`, `\\`))
	drv.commands[mapCmd] = ""

	c := NewTargetClient(drv)
	result, err := c.SetupDiskForMigration(targetName, parentPath, diskIndex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result path")
	}
}

func TestSetupDiskForMigration_EnsureDirFails(t *testing.T) {
	targetName := "forklift-abc123"
	parentPath := `C:\VMs\win2019\disk0.vhdx`

	ensureCmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)

	drv := newMockDriver()
	drv.errors[ensureCmd] = errors.New("access denied")

	c := NewTargetClient(drv)
	_, err := c.SetupDiskForMigration(targetName, parentPath, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetupDiskForMigration_CreateDiskFails(t *testing.T) {
	targetName := "forklift-abc123"
	parentPath := `C:\VMs\win2019\disk0.vhdx`
	diskIndex := 0
	diffPath := ps.DiffDiskPath(targetName, diskIndex)

	ensureCmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffPath)
	createCmd := ps.BuildCommand(ps.NewIscsiVirtualDisk, diffPath, parentPath)

	drv := newMockDriver()
	drv.commands[ensureCmd] = ""
	drv.commands[checkCmd] = ""
	drv.errors[createCmd] = errors.New("parent path not found")

	c := NewTargetClient(drv)
	_, err := c.SetupDiskForMigration(targetName, parentPath, diskIndex)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetupDiskForMigration_MapFails_RollsBackVirtualDisk(t *testing.T) {
	targetName := "forklift-abc123"
	parentPath := `C:\VMs\win2019\disk0.vhdx`
	diskIndex := 0
	diffPath := ps.DiffDiskPath(targetName, diskIndex)

	ensureCmd := ps.BuildCommand(ps.EnsureIscsiTargetDir, ps.IscsiTargetDir)
	checkCmd := ps.BuildCommand(ps.GetIscsiVirtualDisk, diffPath)
	createCmd := ps.BuildCommand(ps.NewIscsiVirtualDisk, diffPath, parentPath)
	mapCmd := ps.BuildCommand(ps.AddIscsiVirtualDiskTargetMapping, targetName, diffPath, "0")
	removeCmd := ps.BuildCommand(ps.RemoveIscsiVirtualDisk, diffPath)

	drv := newMockDriver()
	drv.commands[ensureCmd] = ""
	drv.commands[checkCmd] = ""
	drv.commands[createCmd] = fmt.Sprintf(`{"Path":"%s"}`, strings.ReplaceAll(diffPath, `\`, `\\`))
	drv.errors[mapCmd] = errors.New("target not found")
	drv.commands[removeCmd] = ""

	c := NewTargetClient(drv)
	_, err := c.SetupDiskForMigration(targetName, parentPath, diskIndex)
	if err == nil {
		t.Fatal("expected error from mapping failure")
	}

	rolledBack := false
	for _, cmd := range drv.called {
		if cmd == removeCmd {
			rolledBack = true
			break
		}
	}
	if !rolledBack {
		t.Error("expected rollback of virtual disk after mapping failure")
	}
}

func TestTeardownVM_Success(t *testing.T) {
	targetName := "forklift-abc123"
	pattern := ps.DiffDiskPattern(targetName)

	listCmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)
	rmFiles := ps.BuildCommand(ps.RemoveFilesByPattern, pattern)
	getCmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)
	removeCmd := ps.BuildCommand(ps.RemoveIscsiServerTarget, targetName)

	drv := newMockDriver()
	drv.commands[listCmd] = ""
	drv.commands[rmFiles] = ""
	drv.commands[getCmd] = `{"TargetIqn":"iqn.test","Status":"Connected","LunCount":0}`
	drv.commands[removeCmd] = ""
	drv.hooks[removeCmd] = func(m *mockDriver) {
		m.commands[getCmd] = ""
	}

	c := NewTargetClient(drv)
	if err := c.TeardownVM(targetName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeardownVM_CleanupFails_StillRemovesTarget(t *testing.T) {
	targetName := "forklift-abc123"
	pattern := ps.DiffDiskPattern(targetName)

	listCmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)
	rmFiles := ps.BuildCommand(ps.RemoveFilesByPattern, pattern)
	getCmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)
	removeCmd := ps.BuildCommand(ps.RemoveIscsiServerTarget, targetName)

	drv := newMockDriver()
	drv.errors[listCmd] = errors.New("partial failure")
	drv.errors[rmFiles] = errors.New("partial failure")
	drv.commands[getCmd] = `{"TargetIqn":"iqn.test","Status":"Connected","LunCount":0}`
	drv.commands[removeCmd] = ""
	drv.hooks[removeCmd] = func(m *mockDriver) {
		m.commands[getCmd] = ""
	}

	c := NewTargetClient(drv)
	// CleanupDiffDisks will fail on rmFiles, but TeardownVM logs it and proceeds.
	// RemoveTarget should still succeed.
	if err := c.TeardownVM(targetName); err != nil {
		t.Fatalf("unexpected error: %v (should succeed despite cleanup failure)", err)
	}

	removeCalled := false
	for _, cmd := range drv.called {
		if cmd == removeCmd {
			removeCalled = true
			break
		}
	}
	if !removeCalled {
		t.Error("expected target removal even when cleanup fails")
	}
}

func TestTeardownVM_RemoveTargetFails(t *testing.T) {
	targetName := "forklift-abc123"
	pattern := ps.DiffDiskPattern(targetName)

	listCmd := ps.BuildCommand(ps.GetIscsiVirtualDiskTargetMappings, targetName)
	rmFiles := ps.BuildCommand(ps.RemoveFilesByPattern, pattern)
	getCmd := ps.BuildCommand(ps.GetIscsiTarget, targetName)
	removeCmd := ps.BuildCommand(ps.RemoveIscsiServerTarget, targetName)

	drv := newMockDriver()
	drv.commands[listCmd] = ""
	drv.commands[rmFiles] = ""
	// GetTarget always returns the target (never goes away) — simulates sticky target.
	drv.commands[getCmd] = `{"TargetIqn":"iqn.test","Status":"Connected","LunCount":0}`
	drv.errors[removeCmd] = errors.New("sticky target")

	c := NewTargetClient(drv)
	err := c.TeardownVM(targetName)
	if err == nil {
		t.Fatal("expected error when target removal fails")
	}
}
