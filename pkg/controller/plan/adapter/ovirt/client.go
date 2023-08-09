package ovirt

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/settings"
	ovirtsdk "github.com/ovirt/go-ovirt"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	snapshotDesc = "Forklift Operator warm migration precopy"
)

// VM power states
const (
	powerOn      = "On"
	powerOff     = "Off"
	powerUnknown = "Unknown"
)

// Snapshot event codes
const (
	SNAPSHOT_FINISHED_SUCCESS        int64 = 68
	SNAPSHOT_FINISHED_FAILURE        int64 = 69
	REMOVE_SNAPSHOT_FINISHED_SUCCESS int64 = 356
	REMOVE_SNAPSHOT_FINISHED_FAILURE int64 = 357
)

// oVirt VM Client
type Client struct {
	*plancontext.Context
	connection *ovirtsdk.Connection
}

// Create a VM snapshot and return its ID.
func (r *Client) CreateSnapshot(vmRef ref.Ref) (snapshot string, err error) {
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	correlationID, err := r.getSnapshotCorrelationID(vmRef, nil)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	snapsService := vmService.SnapshotsService()
	snap, err := snapsService.Add().Snapshot(
		ovirtsdk.NewSnapshotBuilder().
			Description(snapshotDesc).
			PersistMemorystate(false).
			MustBuild(),
	).Query("correlation_id", correlationID).Send()
	if err != nil {
		if strings.Contains(err.Error(), "Cannot create Snapshot") {
			err = web.ConflictError{
				Provider: r.Source.Provider,
				Err:      err,
			}
		} else {
			err = liberr.Wrap(err)
		}
		return
	}
	snapshot = snap.MustSnapshot().MustId()
	return
}

// Remove all warm migration snapshots.
func (r *Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) (err error) {
	// Snapshot removal is done in Finalize to avoid race conditions
	return
}

// Check if a snapshot is ready to transfer, to avoid importer restarts.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, snapshot string) (ready bool, err error) {
	correlationID, err := r.getSnapshotCorrelationID(vmRef, &snapshot)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	events, err := r.getEvents(correlationID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(events) < 1 {
		err = liberr.New("No event found for correlation ID", "correlationID", correlationID)
		return
	}
	for _, event := range events {
		code, _ := event.Code()
		switch code {
		case SNAPSHOT_FINISHED_FAILURE:
			err = liberr.New("Snapshot creation failed!", "correlationID", correlationID)
			return
		case SNAPSHOT_FINISHED_SUCCESS:
			ready = true
			return
		}
	}
	return
}

// Set DataVolume checkpoints.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) (err error) {
	n := len(precopies)
	previous := ""
	current := precopies[n-1].Snapshot
	if n >= 2 {
		previous = precopies[n-2].Snapshot
	}

	for i := range datavolumes {
		dv := &datavolumes[i]
		var currentDiskSnapshot, previousDiskSnapshot string
		currentDiskSnapshot, err = r.getDiskSnapshot(dv.Spec.Source.Imageio.DiskID, current)
		if err != nil {
			return
		}
		if previous != "" {
			previousDiskSnapshot, err = r.getDiskSnapshot(dv.Spec.Source.Imageio.DiskID, previous)
			if err != nil {
				return
			}
		}

		dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, cdi.DataVolumeCheckpoint{
			Current:  currentDiskSnapshot,
			Previous: previousDiskSnapshot,
		})
		dv.Spec.FinalCheckpoint = final
	}
	return
}

// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state string, err error) {
	vm, _, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	status, _ := vm.Status()
	switch status {
	case ovirtsdk.VMSTATUS_DOWN:
		state = powerOff
	case ovirtsdk.VMSTATUS_UP, ovirtsdk.VMSTATUS_POWERING_UP:
		state = powerOn
	default:
		state = powerUnknown
	}
	return
}

// Power on the VM.
func (r *Client) PowerOn(vmRef ref.Ref) (err error) {
	vm, vmService, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	// Request the VM startup if VM is not UP
	if status, _ := vm.Status(); status != ovirtsdk.VMSTATUS_UP {
		_, err = vmService.Start().Send()
		if err != nil {
			err = liberr.Wrap(err)
		}
	}
	return
}

// Power off the VM.
func (r *Client) PowerOff(vmRef ref.Ref) (err error) {
	vm, vmService, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	// Request the VM startup if VM is not UP
	if status, _ := vm.Status(); status != ovirtsdk.VMSTATUS_DOWN {
		_, err = vmService.Shutdown().Send()
		if err != nil {
			err = liberr.Wrap(err)
		}
	}
	return
}

// Determine whether the VM has been powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (poweredOff bool, err error) {
	powerState, err := r.PowerState(vmRef)
	if err != nil {
		return
	}
	poweredOff = powerState == powerOff
	return
}

// Close the connection to the oVirt API.
func (r *Client) Close() {
	if r.connection != nil {
		_ = r.connection.Close()
		r.connection = nil
	}
}

// Derive a value from the plan name, the VM ID, and the index of the given
// snapshot in the precopies list. This can be used as the correlation ID for
// tracking the status of a snapshot creation command in the oVirt API. There
// seems to be a limit on correlation ID lengths, so use the MD5 of this value
// as the actual correlation ID. If there is no snapshot ID, assume the last
// snapshot in the precopy list. This way, CreateSnapshot can set the
// correlation ID without knowing the snapshot ID from the oVirt API, because
// the snapshot ID hasn't been created yet.
func (r *Client) getSnapshotCorrelationID(vmRef ref.Ref, snapshot *string) (correlationID string, err error) {
	var vm *planapi.VMStatus
	for _, vmstatus := range r.Migration.Status.VMs {
		if vmstatus.ID == vmRef.ID {
			vm = vmstatus
			break
		}
	}
	if vm == nil {
		err = liberr.New("Could not find VM", "VM", vmRef.ID)
		return
	}
	if vm.Warm == nil {
		err = liberr.New("VM is not part of a warm migration plan", "VM", vmRef.ID)
		return
	}

	precopyIndex := len(vm.Warm.Precopies)
	if snapshot != nil {
		var precopySnapshot *planapi.Precopy
		for index, precopy := range vm.Warm.Precopies {
			if *snapshot == precopy.Snapshot {
				precopySnapshot = &precopy
				precopyIndex = index
				break
			}
		}
		if precopySnapshot == nil {
			err = liberr.New("Could not find snapshot in precopies list", "snapshot", *snapshot)
			return
		}
	}

	uniqueID := fmt.Sprintf("%s-%s-%d", r.Migration.Name, vmRef.ID, precopyIndex)
	hashedID := md5.New()
	hashedID.Write([]byte(uniqueID))
	correlationID = hex.EncodeToString(hashedID.Sum(nil))
	return
}

// Find oVirt jobs with the given correlation ID.
func (r *Client) getEvents(correlationID string) (ovirtJob []*ovirtsdk.Event, err error) {
	eventService := r.connection.SystemService().EventsService().List()
	eventResponse, err := eventService.Search(fmt.Sprintf("correlation_id=%s", correlationID)).Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	ovirtEvent, ok := eventResponse.Events()
	if !ok {
		err = liberr.New("Event source lookup failed", "correlationID", correlationID)
		return
	}
	ovirtJob = ovirtEvent.Slice()
	return
}

// Get the VM by ref.
func (r *Client) getVM(vmRef ref.Ref) (ovirtVm *ovirtsdk.Vm, vmService *ovirtsdk.VmService, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}
	vmService = r.connection.SystemService().VmsService().VmService(vm.ID)
	vmResponse, err := vmService.Get().Query("correlation_id", r.Migration.Name).Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	ovirtVm, ok := vmResponse.Vm()
	if !ok {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s source lookup failed",
				vmRef.String()))
	}
	return
}

// Get the disk snapshot for this disk and this snapshot ID.
func (r *Client) getDiskSnapshot(diskID, targetSnapshotID string) (diskSnapshotID string, err error) {
	response, rErr := r.connection.SystemService().DisksService().DiskService(diskID).Get().Query("correlation_id", r.Migration.Name).Send()
	if err != nil {
		err = liberr.Wrap(rErr)
		return
	}
	disk, ok := response.Disk()
	if !ok {
		err = liberr.New("Could not find disk definition in response.", "disk", diskID)
		return
	}

	storageDomains, ok := disk.StorageDomains()
	if !ok {
		err = liberr.New("No storage domains listed for disk.", "disk", diskID)
		return
	}

	for _, sd := range storageDomains.Slice() {
		sdID, ok := sd.Id()
		if !ok {
			continue
		}
		sdService := r.connection.SystemService().StorageDomainsService().StorageDomainService(sdID)
		if sdService == nil {
			err = liberr.New("No service available for storage domain.", "storageDomain", sdID)
			return
		}
		snapshotsResponse, rErr := sdService.DiskSnapshotsService().List().Send()
		if err != nil {
			err = liberr.Wrap(rErr, "Error listing snapshots in storage domain.", "storageDomain", sdID)
			return
		}
		snapshots, ok := snapshotsResponse.Snapshots()
		if !ok || len(snapshots.Slice()) == 0 {
			err = liberr.New("No snapshots listed in storage domain.", "storageDomain", sdID)
			return
		}
		for _, diskSnapshot := range snapshots.Slice() {
			id, ok := diskSnapshot.Id()
			if !ok {
				continue
			}
			snapshotDisk, ok := diskSnapshot.Disk()
			if !ok {
				continue
			}
			snapshotDiskID, ok := snapshotDisk.Id()
			if !ok {
				continue
			}
			snapshot, ok := diskSnapshot.Snapshot()
			if !ok {
				continue
			}
			sid, ok := snapshot.Id()
			if !ok {
				continue
			}
			if snapshotDiskID == diskID && targetSnapshotID == sid {
				diskSnapshotID = id
				return
			}
		}
	}

	err = liberr.New("Could not find disk snapshot.", "disk", diskID, "vmSnapshot", targetSnapshotID)
	return
}

func (r *Client) isDiskBeingTransferred(diskID string) (bool, error) {
	transfers, err := r.connection.SystemService().ImageTransfersService().List().Send()
	if err != nil {
		err = liberr.Wrap(err)
		return false, err
	}

	for _, transfer := range transfers.MustImageTransfer().Slice() {
		phase, _ := transfer.Phase()
		if phase != ovirtsdk.IMAGETRANSFERPHASE_FINISHED_FAILURE && phase != ovirtsdk.IMAGETRANSFERPHASE_FINISHED_SUCCESS {
			transferDiskID, _ := transfer.MustImage().Id()
			if transferDiskID == diskID {
				return true, nil
			}
		}
	}

	return false, nil
}

// Connect to the oVirt API.
func (r *Client) connect() (err error) {
	URL := r.Source.Provider.Spec.URL
	r.connection, err = ovirtsdk.NewConnectionBuilder().
		URL(URL).
		Username(r.user()).
		Password(r.password()).
		CACert(r.cacert()).
		Insecure(ovirt.GetInsecureSkipVerifyFlag(r.Source.Secret)).
		Build()
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (r *Client) user() string {
	if user, found := r.Source.Secret.Data["user"]; found {
		return string(user)
	}
	return ""
}

func (r *Client) password() string {
	if password, found := r.Source.Secret.Data["password"]; found {
		return string(password)
	}
	return ""
}

func (r *Client) cacert() []byte {
	if cacert, found := r.Source.Secret.Data["cacert"]; found {
		return cacert
	}
	return nil
}

func (r Client) Finalize(vms []*planapi.VMStatus, planName string) {
	defer func() {
		if p := recover(); p != nil {
			r.Log.Info("Recovered from panic:", p)
		}
	}()

	if !r.Plan.Spec.Warm {
		r.Log.Info("Skipping precopy removal for cold migration")
		return
	}

	r.connect()
	defer r.Close()
	var wg sync.WaitGroup
	wg.Add(len(vms))
	for _, vm := range vms {
		_, vmService, err := r.getVM(vm.Ref)
		if err != nil {
			r.Log.Error(err, "Failed to get VM", "vm", vm.Ref.String())
			continue
		}

		go r.removePrecopies(vm.Warm.Precopies, vmService, &wg)
	}

	wg.Wait()
	r.Log.Info("Finished removing precopies")
}

func (r Client) removePrecopies(precopies []planapi.Precopy, vmService *ovirtsdk.VmService, wg *sync.WaitGroup) {
	if len(precopies) == 0 {
		return
	}

	defer wg.Done()
	snapsService := vmService.SnapshotsService()
	for i := range precopies {
		snapshotID := precopies[i].Snapshot
		snapService := snapsService.SnapshotService(snapshotID)
		correlationID := fmt.Sprintf("%s_finalize", snapshotID[0:8])
		cleanupTimeout := time.Now().Add(time.Duration(settings.Settings.Migration.SnapshotRemovalTimeout) * time.Minute)
		for {
			_, err := snapService.Get().Send()
			if err != nil {
				r.Log.Info("The snapshot was removed", "snapshotID", snapshotID)
				break
			}

			// Try to remove the snapshot
			_, err = snapService.Remove().Query("correlation_id", correlationID).Send()
			if err != nil {
				if strings.Contains(err.Error(), "Cannot remove Snapshot") {
					err = web.ConflictError{
						Provider: r.Source.Provider,
						Err:      err,
					}
					r.Log.Error(err, "ConflictError failed to remove snapshot", "snapshotID", snapshotID)
				} else {
					err = liberr.Wrap(err)
					r.Log.Error(err, "Request to remove snapshot failed", "snapshotID", snapshotID)
				}
			} else {
				// Check the events of the snapshot
				finished, err := r.isSnapshotRemovalFinished(correlationID)
				if err != nil {
					err = liberr.Wrap(err)
					r.Log.Error(err, "Error gathering events about the snapshot removal")
				}
				if finished {
					break
				}
			}

			if time.Now().After(cleanupTimeout) {
				r.Log.Info("Timeout waiting for snapshot removal")
				return
			} else {
				time.Sleep(time.Duration(settings.Settings.Migration.SnapshotStatusCheckRate) * time.Second)
			}
		}
	}
}

func (r *Client) isSnapshotRemovalFinished(correlationID string) (finished bool, err error) {
	events, err := r.getEvents(correlationID)
	if err != nil {
		return
	}
	for _, event := range events {
		code, _ := event.Code()
		switch code {
		case REMOVE_SNAPSHOT_FINISHED_FAILURE:
			r.Log.Info("Snapshot removal failed!")
			return true, nil
		case REMOVE_SNAPSHOT_FINISHED_SUCCESS:
			return true, nil
		}
	}
	return
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}
	diskAttachments := vm.DiskAttachments
	for _, da := range diskAttachments {
		if da.Disk.StorageType == "lun" {
			_, err = vmService.DiskAttachmentsService().AttachmentService(da.ID).Remove().Send()
			if err != nil {
				err = liberr.Wrap(
					err,
					"failed to detach LUN disk.",
					"vm",
					vmRef.String(),
					"disk",
					da)
				return
			}
		}
	}
	return
}

func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	ready = true
	return
}
