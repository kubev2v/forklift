package hyperv

import (
	"context"
	"fmt"
	"strings"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var log = logging.WithName("hyperv|client")

// HyperV VM Client
type Client struct {
	*plancontext.Context
	driver driver.HyperVDriver
}

func (r *Client) Close() {
	if r.driver != nil {
		_ = r.driver.Close()
		r.driver = nil
	}
}

func (r *Client) connect() (driver.HyperVDriver, error) {
	if r.driver != nil {
		if alive, _ := r.driver.IsAlive(); alive {
			return r.driver, nil
		}
		_ = r.driver.Close()
		r.driver = nil
	}

	username, password := hvutil.HyperVCredentials(r.Source.Secret)
	host := r.Source.Provider.Spec.URL
	port := hvutil.WinRMPort(r.Source.Provider.Spec.Settings)

	drv := driver.NewWinRMDriver(host, port, username, password, true, nil)
	if err := drv.Connect(); err != nil {
		return nil, fmt.Errorf("WinRM connect failed: %w", err)
	}
	r.driver = drv
	return drv, nil
}

func (r *Client) Finalize(_ []*planapi.VMStatus, _ string) {
	// No source-side cleanup required for HyperV migrations.
}

func (r *Client) DetachDisks(_ ref.Ref) error {
	return nil
}

func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return planapi.VMPowerStateUnknown, err
	}

	drv, err := r.connect()
	if err != nil {
		log.Info("WinRM connect failed, falling back to inventory cache", "vm", vm.Name, "error", err)
		return r.powerStateFromInventory(vm), nil
	}

	// In cluster mode the VM may reside on a different node than the
	// provider entry-point. Route the Get-VM query to the owner node.
	if r.Source.Provider.IsHyperVCluster() && vm.Host != "" {
		cmd := ps.BuildCommand(ps.GetVMState, vm.Name)
		out, err := drv.RunOnNode(cmd, vm.Host)
		if err != nil {
			log.Info("RunOnNode Get-VM failed, falling back to inventory cache",
				"vm", vm.Name, "node", vm.Host, "error", err)
			return r.powerStateFromInventory(vm), nil
		}
		return r.parseStateString(out), nil
	}

	domain, err := drv.LookupDomainByName(vm.Name)
	if err != nil {
		log.Info("VM not found via WinRM, falling back to inventory cache", "vm", vm.Name, "error", err)
		return r.powerStateFromInventory(vm), nil
	}
	defer func() { _ = domain.Free() }()

	state, _, err := domain.GetState()
	if err != nil {
		log.Info("Failed to get VM state via WinRM, falling back to inventory cache", "vm", vm.Name, "error", err)
		return r.powerStateFromInventory(vm), nil
	}

	switch state {
	case driver.DOMAIN_RUNNING:
		return planapi.VMPowerStateOn, nil
	case driver.DOMAIN_SHUTOFF:
		return planapi.VMPowerStateOff, nil
	default:
		return planapi.VMPowerStateUnknown, nil
	}
}

// parseStateString maps the textual output of PowerShell's VM State property
// (e.g. "Off", "Running") to the plan VMPowerState enum.
func (r *Client) parseStateString(raw string) planapi.VMPowerState {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch {
	case strings.Contains(s, "off"):
		return planapi.VMPowerStateOff
	case strings.Contains(s, "running"):
		return planapi.VMPowerStateOn
	default:
		log.Info("Unrecognised VM state string from WinRM", "raw", raw)
		return planapi.VMPowerStateUnknown
	}
}

func (r *Client) powerStateFromInventory(vm *hyperv.VM) planapi.VMPowerState {
	switch vm.PowerState {
	case model.PowerStateOn:
		return planapi.VMPowerStateOn
	case model.PowerStateOff:
		return planapi.VMPowerStateOff
	default:
		return planapi.VMPowerStateUnknown
	}
}

func (r *Client) PowerOn(_ ref.Ref) error {
	return nil
}

func (r *Client) PowerOff(vmRef ref.Ref) error {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return err
	}

	if vm.PowerState == model.PowerStateOff {
		log.Info("VM already powered off", "vm", vm.Name)
		return nil
	}

	drv, err := r.connect()
	if err != nil {
		return err
	}

	// In cluster mode, the VM may be on a different node than the entry
	// point. Route the Stop-VM command to the correct node via RunOnNode.
	if r.Source.Provider.IsHyperVCluster() && vm.Host != "" {
		cmd := ps.BuildCommand(ps.StopVM, vm.Name)
		if _, err = drv.RunOnNode(cmd, vm.Host); err != nil {
			return fmt.Errorf("failed to power off VM %s on node %s: %w", vm.Name, vm.Host, err)
		}
		log.Info("Powered off VM via RunOnNode", "vm", vm.Name, "node", vm.Host)
		return nil
	}

	domain, err := drv.LookupDomainByName(vm.Name)
	if err != nil {
		log.Info("VM not found on provider, treating as already off", "vm", vm.Name)
		return nil
	}
	defer func() { _ = domain.Free() }()

	state, _, err := domain.GetState()
	if err != nil {
		return fmt.Errorf("failed to get VM state: %w", err)
	}
	if state == driver.DOMAIN_SHUTOFF {
		log.Info("VM already powered off (confirmed via WinRM)", "vm", vm.Name)
		return nil
	}

	if err := domain.Shutdown(context.TODO()); err != nil {
		return fmt.Errorf("failed to power off VM %s: %w", vm.Name, err)
	}

	log.Info("Powered off VM", "vm", vm.Name)
	return nil
}

func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	state, err := r.PowerState(vmRef)
	if err != nil {
		return false, err
	}
	return state == planapi.VMPowerStateOff, nil
}

func (r *Client) CreateSnapshot(_ ref.Ref, _ util.HostsFunc) (string, string, error) {
	return "", "", nil
}

func (r *Client) RemoveSnapshot(_ ref.Ref, _ string, _ util.HostsFunc) (string, error) {
	return "", nil
}

func (r *Client) CheckSnapshotReady(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, string, error) {
	return true, "", nil
}

func (r *Client) CheckSnapshotRemove(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, error) {
	return true, nil
}

func (r *Client) SetCheckpoints(_ ref.Ref, _ []planapi.Precopy, _ []cdi.DataVolume, _ bool, _ util.HostsFunc) error {
	return nil
}

func (r *Client) PreTransferActions(_ ref.Ref) (bool, error) {
	return true, nil
}

func (r *Client) GetSnapshotDeltas(_ ref.Ref, _ string, _ util.HostsFunc) (s map[string]string, err error) {
	return
}
