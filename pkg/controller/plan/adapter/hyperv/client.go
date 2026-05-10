package hyperv

import (
	"context"
	"fmt"
	"net"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/iscsi"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logging.WithName("hyperv|client")

const iscsiDefaultPort = "3260"

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

func (r *Client) Finalize(vms []*planapi.VMStatus, _ string) {
	if r.Source.Provider.GetHyperVTransferMethod() != api.HyperVTransferMethodISCSI {
		return
	}

	for _, vm := range vms {
		targetName := iscsiTargetName(vm.Ref.ID)
		drv, err := r.connect()
		if err != nil {
			log.Error(err, "WinRM connect failed during finalize, skipping VM",
				"vm", vm.Name, "target", targetName)
			continue
		}
		tc := iscsi.NewTargetClient(drv)
		if err := tc.TeardownVM(targetName); err != nil {
			log.Error(err, "Failed to teardown iSCSI target during finalize",
				"vm", vm.Name, "target", targetName)
		} else {
			log.Info("Tore down iSCSI target during finalize", "vm", vm.Name, "target", targetName)
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

	switch vm.PowerState {
	case model.PowerStateOn:
		return planapi.VMPowerStateOn, nil
	case model.PowerStateOff:
		return planapi.VMPowerStateOff, nil
	default:
		return planapi.VMPowerStateUnknown, nil
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

// PreTransferActions sets up the iSCSI infrastructure on the Hyper-V host.
// Called during PhaseCreateDataVolumes (after VM is powered off).
//
// Steps:
//  1. Create iSCSI target with IQN-based initiator ACL
//  2. Create differencing disks for each VHDX and map as LUNs
//
// The copy pod (created by the populator framework) will log in and read
// the LUNs as raw block devices.
func (r *Client) PreTransferActions(vmRef ref.Ref) (bool, error) {
	if r.Source.Provider.GetHyperVTransferMethod() != api.HyperVTransferMethodISCSI {
		return true, nil
	}
	vm := &hyperv.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return false, fmt.Errorf("VM lookup failed: %w", err)
	}

	targetName := iscsiTargetName(vmRef.ID)
	initiatorIQN := iscsiInitiatorIQN(r.Context)

	drv, err := r.connect()
	if err != nil {
		return false, fmt.Errorf("WinRM connect failed for iSCSI setup: %w", err)
	}

	tc := iscsi.NewTargetClient(drv)

	targetResult, err := tc.CreateTarget(targetName, initiatorIQN)
	if err != nil {
		return false, fmt.Errorf("failed to create iSCSI target %q for VM %s: %w",
			targetName, vm.Name, err)
	}

	if targetResult.Created {
		log.Info("Created iSCSI target with IQN-based ACL",
			"vm", vm.Name, "target", targetName,
			"targetIQN", targetResult.TargetIQN, "initiatorIQN", initiatorIQN,
			"initiatorIds", targetResult.InitiatorIds)
	} else {
		log.Info("iSCSI target already exists, ACL updated",
			"vm", vm.Name, "target", targetName,
			"targetIQN", targetResult.TargetIQN, "initiatorIQN", initiatorIQN,
			"initiatorIds", targetResult.InitiatorIds)
	}

	if err := mapDisksToISCSI(tc, targetName, vm); err != nil {
		return false, err
	}

	targetStatus, err := tc.GetTarget(targetName)
	if err != nil {
		log.Info("Could not verify target status after LUN setup (non-fatal)", "error", err)
	} else if targetStatus != nil {
		log.Info("Target status after LUN setup",
			"target", targetName, "status", targetStatus.Status,
			"lunCount", targetStatus.LunCount, "targetIQN", targetStatus.TargetIQN)
	}

	// Persist the real target IQN (assigned by Windows) on the Migration so the
	// builder can read it when creating populator CRs.
	annKey := iscsiTargetIQNAnnKey(vmRef.ID)
	migration := r.Migration.DeepCopy()
	if migration.Annotations == nil {
		migration.Annotations = make(map[string]string)
	}
	if migration.Annotations[annKey] != targetResult.TargetIQN {
		migration.Annotations[annKey] = targetResult.TargetIQN
		if err := r.Client.Update(context.TODO(), migration, &client.UpdateOptions{}); err != nil {
			return false, fmt.Errorf("store iSCSI target IQN on migration: %w", err)
		}
		r.Migration.Annotations = migration.Annotations
		log.Info("Stored real target IQN on migration", "key", annKey, "iqn", targetResult.TargetIQN)
	}

	return true, nil
}

func mapDisksToISCSI(tc *iscsi.TargetClient, targetName string, vm *hyperv.VM) error {
	mappedDisks := 0
	for i, disk := range vm.Disks {
		if disk.WindowsPath == "" {
			continue
		}
		mappedDisks++
		diffPath, diskErr := tc.SetupDiskForMigration(targetName, disk.WindowsPath, i)
		if diskErr != nil {
			log.Error(diskErr, "Disk setup failed, cleaning up iSCSI target",
				"vm", vm.Name, "disk", i)
			if cleanupErr := tc.TeardownVM(targetName); cleanupErr != nil {
				log.Error(cleanupErr, "Cleanup after partial failure also failed",
					"target", targetName)
			}
			return fmt.Errorf("failed to setup iSCSI diff disk for %s disk %d: %w",
				vm.Name, i, diskErr)
		}
		log.Info("Mapped VHDX as iSCSI LUN",
			"vm", vm.Name, "disk", i,
			"parentVhdx", disk.WindowsPath, "diffDisk", diffPath,
			"lun", i)
	}
	if mappedDisks == 0 {
		return fmt.Errorf("no migratable disks found for VM %s", vm.Name)
	}
	return nil
}

func (r *Client) GetSnapshotDeltas(_ ref.Ref, _ string, _ util.HostsFunc) (s map[string]string, err error) {
	return
}

// iscsiTargetIQNAnnKey returns the annotation key used to store the iSCSI
// target IQN for a specific VM on the Migration object.
// Example: "forklift.konveyor.io/iscsi-iqn.5C2A1B3D-..."
func iscsiTargetIQNAnnKey(vmID string) string {
	return "forklift.konveyor.io/iscsi-iqn." + vmID
}

// iscsiTargetName builds the Windows iSCSI target name for a VM.
func iscsiTargetName(vmID string) string {
	id := strings.ReplaceAll(vmID, "-", "")
	if len(id) > 40 {
		id = id[:40]
	}
	return "forklift-" + id
}

// iscsiInitiatorIQN returns the unique IQN the copy pod will use to
// authenticate against the target's initiator ACL for this migration.
func iscsiInitiatorIQN(ctx *plancontext.Context) string {
	migrationUID := string(ctx.Migration.GetUID())
	if migrationUID == "" {
		migrationUID = "unknown"
	}
	created := ctx.Migration.GetCreationTimestamp().Format("2006-01")
	return fmt.Sprintf("iqn.%s.io.forklift:copy-%s", created, migrationUID)
}

// iscsiTargetPortal returns the iSCSI portal address for the Hyper-V provider.
func iscsiTargetPortal(ctx *plancontext.Context) string {
	host := strings.TrimSpace(ctx.Source.Provider.Spec.URL)
	// Try to parse as host:port (handles IPv6 bracket notation).
	if h, _, err := net.SplitHostPort(host); err == nil {
		return net.JoinHostPort(h, iscsiDefaultPort)
	}
	// No port present — just append the iSCSI port.
	return net.JoinHostPort(host, iscsiDefaultPort)
}
