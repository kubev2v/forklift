package hyperv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
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

const clusterPowerOffTimeout = 5 * time.Minute

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// WinRM may wrap the deadline in a plain message instead of chaining it.
	return strings.Contains(err.Error(), "context deadline exceeded")
}

// Client talks to Hyper-V over WinRM for plan operations (power, checkpoints).
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

func (r *Client) Finalize(vms []*planapi.VMStatus, _ string) {
	if !r.Plan.IsWarm() {
		return
	}
	// Own connection: Finalize runs in a goroutine while Migration may Close
	// the shared driver as soon as Run returns.
	username, password := hvutil.HyperVCredentials(r.Source.Secret)
	host := r.Source.Provider.Spec.URL
	port := hvutil.WinRMPort(r.Source.Provider.Spec.Settings)
	drv := driver.NewWinRMDriver(host, port, username, password, true, nil)
	if err := drv.Connect(); err != nil {
		log.Error(err, "Failed to connect for checkpoint cleanup")
		return
	}
	defer func() { _ = drv.Close() }()

	// Safety net after PhaseRemoveFinalSnapshot. Clears leftover forklift
	// checkpoints if the migration failed before that phase.
	for _, vm := range vms {
		vmLookup := &hyperv.VM{}
		if err := r.Source.Inventory.Find(vmLookup, vm.Ref); err != nil {
			log.Error(err, "Failed to find VM for checkpoint cleanup", "vm", vm.Name)
			continue
		}
		cmd := ps.BuildCommand(ps.RemoveStaleForkliftCheckpoints, vmLookup.Name)
		if out, err := drv.RunOnNode(cmd, vmLookup.Host); err != nil {
			log.Info("Finalize checkpoint cleanup failed", "vm", vmLookup.Name, "error", err)
		} else if out != "" {
			log.Info("Finalize cleaned up remaining checkpoints", "vm", vmLookup.Name, "removed", out)
		}
	}
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

	// Cluster: VM may live on a different node than the provider entry point.
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

// parseStateString maps PowerShell VM State text ("Off", "Running") to VMPowerState.
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

	// Cluster: Stop-VM on the owner node with a longer timeout (multi-hop WinRM).
	if r.Source.Provider.IsHyperVCluster() && vm.Host != "" {
		cmd := ps.BuildCommand(ps.StopVM, vm.Name)
		if _, err = drv.RunOnNodeWithTimeout(cmd, vm.Host, clusterPowerOffTimeout); err != nil {
			// Timeout is non-fatal: VMMS often accepts Stop-VM -Force before
			// WinRM returns. Advance to WaitForPowerOff, which polls until off.
			if isTimeoutError(err) {
				log.Info("Stop-VM timed out, deferring to WaitForPowerOff",
					"vm", vm.Name, "node", vm.Host, "error", err)
				return nil
			}
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

func (r *Client) CreateSnapshot(vmRef ref.Ref, _ util.HostsFunc) (string, string, error) {
	if !r.Plan.IsWarm() {
		return "", "", nil
	}
	vm := &hyperv.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return "", "", err
	}
	drv, err := r.connect()
	if err != nil {
		return "", "", err
	}
	r.cleanupStaleWarmCheckpoints(drv, vm)
	cpName := r.warmCheckpointName(vmRef)
	cmd := ps.BuildCommand(ps.CreateVMCheckpoint, vm.Name, cpName)
	if _, err = drv.RunOnNode(cmd, vm.Host); err != nil {
		return "", "", fmt.Errorf("creating checkpoint on %s: %w", vm.Name, err)
	}
	log.Info("Created checkpoint", "vm", vm.Name, "checkpoint", cpName)
	return cpName, cpName, nil
}

// cleanupStaleWarmCheckpoints drops leftover forklift-* checkpoints before the
// first precopy of a migration (e.g. after a controller restart).
func (r *Client) cleanupStaleWarmCheckpoints(drv driver.HyperVDriver, vm *hyperv.VM) {
	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.ID == vm.ID && vmStatus.Warm != nil && len(vmStatus.Warm.Precopies) > 0 {
			return
		}
	}
	cleanCmd := ps.BuildCommand(ps.RemoveStaleForkliftCheckpoints, vm.Name)
	if out, cleanErr := drv.RunOnNode(cleanCmd, vm.Host); cleanErr != nil {
		log.Info("Stale checkpoint cleanup failed (non-fatal)", "vm", vm.Name, "error", cleanErr)
	} else if out != "" {
		log.Info("Cleaned up stale checkpoints", "vm", vm.Name, "removed", out)
	}
}

// warmCheckpointName returns forklift-warm-<mig8>-<precopyIndex>.
func (r *Client) warmCheckpointName(vmRef ref.Ref) string {
	mig := "nomig"
	if r.Migration != nil && r.Migration.UID != "" {
		mig = string(r.Migration.UID)
		if len(mig) > 8 {
			mig = mig[:8]
		}
	}
	idx := 0
	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.ID == vmRef.ID && vmStatus.Warm != nil {
			idx = len(vmStatus.Warm.Precopies)
			break
		}
	}
	return fmt.Sprintf("forklift-warm-%s-%d", mig, idx)
}

func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshotName string, _ util.HostsFunc) (string, error) {
	if !r.Plan.IsWarm() {
		return "", nil
	}

	// Only PhaseRemoveFinalSnapshot deletes checkpoints. Earlier removals
	// are no-ops so the stacked AVHDX chain can keep tracking deltas.
	isFinal := false
	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.ID == vmRef.ID {
			isFinal = vmStatus.Phase == api.PhaseRemoveFinalSnapshot
			break
		}
	}
	if !isFinal {
		log.Info("Skipping checkpoint removal (stacked for delta tracking)", "checkpoint", snapshotName)
		return "", nil
	}

	vm := &hyperv.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return "", err
	}
	drv, err := r.connect()
	if err != nil {
		return "", err
	}
	// Best-effort after disk copy; Client.Finalize retries if this fails.
	cleanCmd := ps.BuildCommand(ps.RemoveStaleForkliftCheckpoints, vm.Name)
	if out, err := drv.RunOnNode(cleanCmd, vm.Host); err != nil {
		log.Info("Final checkpoint cleanup failed (non-fatal)", "vm", vm.Name, "error", err)
	} else if out != "" {
		log.Info("Removed all forklift checkpoints", "vm", vm.Name, "removed", out)
	}
	return "", nil
}

func (r *Client) CheckSnapshotReady(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, string, error) {
	// Standard checkpoints complete synchronously.
	return true, "", nil
}

func (r *Client) CheckSnapshotRemove(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, error) {
	// AVHDX merge after removal is async in the background. Safe to continue
	// without waiting (active disk chain is unaffected).
	return true, nil
}

func (r *Client) SetCheckpoints(_ ref.Ref, _ []planapi.Precopy, _ []cdi.DataVolume, _ bool, _ util.HostsFunc) error {
	return nil
}

func (r *Client) PreTransferActions(_ ref.Ref) (bool, error) {
	return true, nil
}

// GetSnapshotDeltas returns per-disk AVHDX path JSON after a checkpoint.
// The warm-copy binary reads changed ranges from those AVHDX files (like CBT).
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, _ string, _ util.HostsFunc) (map[string]string, error) {
	if !r.Plan.IsWarm() {
		return map[string]string{}, nil
	}
	vm := &hyperv.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return nil, err
	}
	drv, err := r.connect()
	if err != nil {
		return nil, err
	}
	cmd := ps.BuildCommand(ps.GetCheckpointDiskPaths, vm.Name)
	stdout, err := drv.RunOnNode(cmd, vm.Host)
	if err != nil {
		return nil, fmt.Errorf("getting checkpoint disk paths for %s: %w", vm.Name, err)
	}
	// PowerShell: one disk → object, many → array.
	raw := strings.TrimSpace(stdout)
	var diskPaths []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &diskPaths); err != nil {
		diskPaths = []json.RawMessage{json.RawMessage(raw)}
	}

	deltas := map[string]string{}
	for _, rec := range diskPaths {
		var info struct {
			Active string   `json:"Active"`
			Parent string   `json:"Parent"`
			Root   string   `json:"Root"`
			Chain  []string `json:"Chain"`
		}
		if err := json.Unmarshal(rec, &info); err != nil {
			return nil, fmt.Errorf("parsing checkpoint disk paths for %s: %w", vm.Name, err)
		}
		diskID := ""
		for _, disk := range vm.Disks {
			// Match Root/Parent/Active or any Chain entry — inventory may still
			// name a mid-chain AVHDX after later checkpoints reparent Active.
			if checkpointDiskMatchesInventory(disk.WindowsPath, info.Root, info.Parent, info.Active, info.Chain) {
				diskID = disk.ID
				break
			}
		}
		if diskID == "" {
			return nil, fmt.Errorf("checkpoint disk %s of VM %s matches no inventory disk",
				info.Root, vm.Name)
		}
		if _, dup := deltas[diskID]; dup {
			return nil, fmt.Errorf("checkpoint disk %s of VM %s matches inventory disk %s twice",
				info.Root, vm.Name, diskID)
		}
		deltas[diskID] = fmt.Sprintf("{\"diskPaths\":%s}", string(rec))
	}
	// Every inventory disk must have a delta or the copy pod would leave a blank PVC.
	for _, disk := range vm.Disks {
		if _, found := deltas[disk.ID]; !found {
			return nil, fmt.Errorf("no checkpoint delta found for disk %s (%s) of VM %s",
				disk.ID, disk.WindowsPath, vm.Name)
		}
	}
	return deltas, nil
}

// sameWindowsPath compares Windows paths case-insensitively (\\ and / normalized).
func sameWindowsPath(a, b string) bool {
	norm := func(p string) string {
		return strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
	}
	return norm(a) != "" && norm(a) == norm(b)
}

// checkpointDiskMatchesInventory reports whether inventoryPath is the Root,
// Parent, Active, or any ancestor in Chain for a checkpoint disk record.
func checkpointDiskMatchesInventory(inventoryPath, root, parent, active string, chain []string) bool {
	if sameWindowsPath(inventoryPath, root) ||
		sameWindowsPath(inventoryPath, parent) ||
		sameWindowsPath(inventoryPath, active) {
		return true
	}
	for _, p := range chain {
		if sameWindowsPath(inventoryPath, p) {
			return true
		}
	}
	return false
}
