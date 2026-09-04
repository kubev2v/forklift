package hyperv

import (
	"errors"
	"testing"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
)

func TestParseStateString(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name     string
		raw      string
		expected planapi.VMPowerState
	}{
		{"off lowercase", "Off", planapi.VMPowerStateOff},
		{"off with whitespace", "  Off\r\n", planapi.VMPowerStateOff},
		{"running", "Running", planapi.VMPowerStateOn},
		{"running with whitespace", "\n Running \n", planapi.VMPowerStateOn},
		{"unknown state", "Paused", planapi.VMPowerStateUnknown},
		{"empty string", "", planapi.VMPowerStateUnknown},
		{"shutoff variant", "TurnedOff", planapi.VMPowerStateOff},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.parseStateString(tc.raw)
			if got != tc.expected {
				t.Errorf("parseStateString(%q) = %q, want %q", tc.raw, got, tc.expected)
			}
		})
	}
}

func TestPowerStateFromInventory(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name     string
		state    string
		expected planapi.VMPowerState
	}{
		{"on", model.PowerStateOn, planapi.VMPowerStateOn},
		{"off", model.PowerStateOff, planapi.VMPowerStateOff},
		{"paused", model.PowerStatePaused, planapi.VMPowerStateUnknown},
		{"empty", "", planapi.VMPowerStateUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := &hyperv.VM{PowerState: tc.state}
			got := c.powerStateFromInventory(vm)
			if got != tc.expected {
				t.Errorf("powerStateFromInventory(state=%q) = %q, want %q",
					tc.state, got, tc.expected)
			}
		})
	}
}

// stubDriver is a minimal driver.HyperVDriver implementation for testing
// Client.PowerOff without a real WinRM connection. Only RunOnNode is
// exercised by PowerOff; the rest satisfy the interface and are unused.
type stubDriver struct {
	runOnNodeCmd  string
	runOnNodeHost string
	runOnNodeErr  error
	runOnNodeN    int
}

func (s *stubDriver) Connect() error { return nil }
func (s *stubDriver) Close() error   { return nil }
func (s *stubDriver) IsAlive() (bool, error) {
	return true, nil
}
func (s *stubDriver) ListAllDomains() ([]driver.Domain, error)        { return nil, nil }
func (s *stubDriver) ListAllClusterDomains() ([]driver.Domain, error) { return nil, nil }
func (s *stubDriver) LookupDomainByName(_ string) (driver.Domain, error) {
	return nil, errors.New("not implemented")
}
func (s *stubDriver) LookupDomainByUUIDString(_ string) (driver.Domain, error) {
	return nil, errors.New("not implemented")
}
func (s *stubDriver) ListAllNetworks() ([]driver.Network, error) { return nil, nil }
func (s *stubDriver) LookupNetworkByUUIDString(_ string) (driver.Network, error) {
	return nil, errors.New("not implemented")
}
func (s *stubDriver) GetCluster() (*driver.ClusterData, error)           { return nil, nil }
func (s *stubDriver) GetClusterNodes() ([]driver.ClusterNodeData, error) { return nil, nil }
func (s *stubDriver) GetClusterInfo() (*driver.ClusterInfoData, error)   { return nil, nil }
func (s *stubDriver) GetClusterVMGroups() ([]driver.ClusterGroupData, error) {
	return nil, nil
}
func (s *stubDriver) GetComputerInfo() (*driver.ComputerInfoData, error) { return nil, nil }
func (s *stubDriver) ExecuteCommand(_ string) (string, error)            { return "", nil }

func (s *stubDriver) RunOnNode(command, computerName string) (string, error) {
	s.runOnNodeN++
	s.runOnNodeCmd = command
	s.runOnNodeHost = computerName
	if s.runOnNodeErr != nil {
		return "", s.runOnNodeErr
	}
	return "", nil
}

func makePowerVM(id, host, powerState string) *hyperv.VM {
	vm := makeVM(id, host)
	vm.Name = id
	vm.PowerState = powerState
	return vm
}

func TestPowerOff_AlreadyOff(t *testing.T) {
	inv := &stubInventory{
		vms: map[string]*hyperv.VM{"vm-1": makePowerVM("vm-1", "node-1", model.PowerStateOff)},
	}
	drv := &stubDriver{}
	c := &Client{
		Context: &plancontext.Context{
			Source: plancontext.Source{Inventory: inv},
		},
		driver: drv,
	}

	if err := c.PowerOff(ref.Ref{ID: "vm-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if drv.runOnNodeN != 0 {
		t.Errorf("expected RunOnNode not to be called, called %d times", drv.runOnNodeN)
	}
}

func TestPowerOff_RunsOnNode(t *testing.T) {
	inv := &stubInventory{
		vms: map[string]*hyperv.VM{"vm-1": makePowerVM("vm-1", "node-1", model.PowerStateOn)},
	}
	drv := &stubDriver{}
	c := &Client{
		Context: &plancontext.Context{
			Source: plancontext.Source{Inventory: inv},
		},
		driver: drv,
	}

	if err := c.PowerOff(ref.Ref{ID: "vm-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantCmd := ps.BuildCommand(ps.InitiateShutdown, "vm-1")
	if drv.runOnNodeCmd != wantCmd {
		t.Errorf("RunOnNode command = %q, want %q", drv.runOnNodeCmd, wantCmd)
	}
	if drv.runOnNodeHost != "node-1" {
		t.Errorf("RunOnNode host = %q, want %q", drv.runOnNodeHost, "node-1")
	}
}

func TestPowerOff_RunOnNodeError(t *testing.T) {
	inv := &stubInventory{
		vms: map[string]*hyperv.VM{"vm-1": makePowerVM("vm-1", "node-1", model.PowerStateOn)},
	}
	drv := &stubDriver{runOnNodeErr: errors.New("boom")}
	c := &Client{
		Context: &plancontext.Context{
			Source: plancontext.Source{Inventory: inv},
		},
		driver: drv,
	}

	err := c.PowerOff(ref.Ref{ID: "vm-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	wantMsg := "failed to initiate shutdown for VM vm-1: boom"
	if err.Error() != wantMsg {
		t.Errorf("error = %q, want %q", err.Error(), wantMsg)
	}
}

func TestPowerOff_EmptyHostStandalone(t *testing.T) {
	inv := &stubInventory{
		vms: map[string]*hyperv.VM{"vm-1": makePowerVM("vm-1", "", model.PowerStateOn)},
	}
	drv := &stubDriver{}
	c := &Client{
		Context: &plancontext.Context{
			Source: plancontext.Source{Inventory: inv},
		},
		driver: drv,
	}

	if err := c.PowerOff(ref.Ref{ID: "vm-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if drv.runOnNodeN != 1 {
		t.Errorf("expected RunOnNode to be called once, called %d times", drv.runOnNodeN)
	}
	if drv.runOnNodeHost != "" {
		t.Errorf("RunOnNode host = %q, want empty string", drv.runOnNodeHost)
	}
}
