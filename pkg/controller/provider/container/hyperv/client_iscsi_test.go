package hyperv

import (
	"errors"
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var errNotImplemented = errors.New("not implemented in mock")

type mockDriver struct {
	commands map[string]string
	errors   map[string]error
}

func (m *mockDriver) Connect() error         { return nil }
func (m *mockDriver) Close() error           { return nil }
func (m *mockDriver) IsAlive() (bool, error) { return true, nil }
func (m *mockDriver) ListAllDomains() ([]driver.Domain, error) {
	return nil, errNotImplemented
}
func (m *mockDriver) LookupDomainByName(string) (driver.Domain, error) {
	return nil, errNotImplemented
}
func (m *mockDriver) LookupDomainByUUIDString(string) (driver.Domain, error) {
	return nil, errNotImplemented
}
func (m *mockDriver) ListAllNetworks() ([]driver.Network, error) {
	return nil, errNotImplemented
}
func (m *mockDriver) LookupNetworkByUUIDString(string) (driver.Network, error) {
	return nil, errNotImplemented
}

func (m *mockDriver) ExecuteCommand(command string) (string, error) {
	if e, ok := m.errors[command]; ok {
		return "", e
	}
	if out, ok := m.commands[command]; ok {
		return out, nil
	}
	return "", fmt.Errorf("unexpected command: %s", command)
}

func TestCheckIscsiReadiness_AllReady(t *testing.T) {
	client := &Client{
		Log: logging.WithName("test"),
		driver: &mockDriver{
			commands: map[string]string{
				ps.CheckIscsiTargetFeature: `{"Installed":true}`,
				ps.CheckIscsiFirewallPort:  `{"Open":true}`,
			},
		},
	}

	result, err := client.CheckIscsiReadiness()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.FeatureInstalled {
		t.Error("expected FeatureInstalled to be true")
	}
	if !result.FirewallOpen {
		t.Error("expected FirewallOpen to be true")
	}
}

func TestCheckIscsiReadiness_FeatureNotInstalled(t *testing.T) {
	client := &Client{
		Log: logging.WithName("test"),
		driver: &mockDriver{
			commands: map[string]string{
				ps.CheckIscsiTargetFeature: `{"Installed":false}`,
			},
		},
	}

	result, err := client.CheckIscsiReadiness()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FeatureInstalled {
		t.Error("expected FeatureInstalled to be false")
	}
	if result.FirewallOpen {
		t.Error("expected FirewallOpen to be false when feature not installed")
	}
}

func TestCheckIscsiReadiness_FeatureInstalledPortClosed(t *testing.T) {
	client := &Client{
		Log: logging.WithName("test"),
		driver: &mockDriver{
			commands: map[string]string{
				ps.CheckIscsiTargetFeature: `{"Installed":true}`,
				ps.CheckIscsiFirewallPort:  `{"Open":false}`,
			},
		},
	}

	result, err := client.CheckIscsiReadiness()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.FeatureInstalled {
		t.Error("expected FeatureInstalled to be true")
	}
	if result.FirewallOpen {
		t.Error("expected FirewallOpen to be false")
	}
}

func TestCheckIscsiReadiness_FeatureCheckFails(t *testing.T) {
	client := &Client{
		Log: logging.WithName("test"),
		driver: &mockDriver{
			commands: map[string]string{},
			errors: map[string]error{
				ps.CheckIscsiTargetFeature: fmt.Errorf("WinRM timeout"),
			},
		},
	}

	result, err := client.CheckIscsiReadiness()
	if err == nil {
		t.Fatal("expected error on WinRM failure, got nil")
	}
	if result.FeatureInstalled {
		t.Error("expected FeatureInstalled to be false on command failure")
	}
}

func TestCheckIscsiReadiness_PortCheckFails(t *testing.T) {
	client := &Client{
		Log: logging.WithName("test"),
		driver: &mockDriver{
			commands: map[string]string{
				ps.CheckIscsiTargetFeature: `{"Installed":true}`,
			},
			errors: map[string]error{
				ps.CheckIscsiFirewallPort: fmt.Errorf("WinRM timeout"),
			},
		},
	}

	result, err := client.CheckIscsiReadiness()
	if err == nil {
		t.Fatal("expected error on WinRM failure, got nil")
	}
	if !result.FeatureInstalled {
		t.Error("expected FeatureInstalled to be true")
	}
	if result.FirewallOpen {
		t.Error("expected FirewallOpen to be false on command failure")
	}
}

func TestCheckIscsiReadiness_NoDriver(t *testing.T) {
	client := &Client{
		Log:    logging.WithName("test"),
		driver: nil,
	}

	_, err := client.CheckIscsiReadiness()
	if err == nil {
		t.Error("expected error when driver is nil")
	}
}

func newProviderWithTransferMethod(method string) *api.Provider {
	p := &api.Provider{}
	p.Spec.Settings = map[string]string{
		api.HyperVTransferMethod: method,
	}
	return p
}

func TestListStorages_ISCSI(t *testing.T) {
	client := &Client{
		Log:      logging.WithName("test"),
		provider: newProviderWithTransferMethod("iscsi"),
	}

	storages, err := client.ListStorages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(storages) != 1 {
		t.Fatalf("expected 1 storage, got %d", len(storages))
	}
	if storages[0].ID != hvutil.StorageIDDefault {
		t.Errorf("expected ID %q, got %q", hvutil.StorageIDDefault, storages[0].ID)
	}
	if storages[0].Type != StorageTypeISCSI {
		t.Errorf("expected Type %q, got %q", StorageTypeISCSI, storages[0].Type)
	}
	if storages[0].Name != StorageNameISCSI {
		t.Errorf("expected Name %q, got %q", StorageNameISCSI, storages[0].Name)
	}
}

func TestListStorages_SMB_NoURL(t *testing.T) {
	client := &Client{
		Log:      logging.WithName("test"),
		provider: newProviderWithTransferMethod("smb"),
		smbUrl:   "",
	}

	storages, err := client.ListStorages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storages != nil {
		t.Errorf("expected nil storages for SMB provider with no URL, got %v", storages)
	}
}

func TestListStorages_SMB_WithURL(t *testing.T) {
	client := &Client{
		Log:      logging.WithName("test"),
		provider: newProviderWithTransferMethod("smb"),
		smbUrl:   "smb://host/share",
	}

	storages, err := client.ListStorages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(storages) != 1 {
		t.Fatalf("expected 1 storage, got %d", len(storages))
	}
	if storages[0].Type != StorageTypeSMB {
		t.Errorf("expected Type %q, got %q", StorageTypeSMB, storages[0].Type)
	}
}
