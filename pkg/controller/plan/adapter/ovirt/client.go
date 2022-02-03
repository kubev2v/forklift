package ovirt

import (
	"fmt"

	liberr "github.com/konveyor/controller/pkg/error"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	ovirtsdk "github.com/ovirt/go-ovirt"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	snapshotName = "forklift-migration-precopy"
	snapshotDesc = "Forklift Operator warm migration precopy"
)

// VM power states
const (
	powerOn      = "On"
	powerOff     = "Off"
	powerUnknown = "Unknown"
)

//
// oVirt VM Client
type Client struct {
	*plancontext.Context
	connection *ovirtsdk.Connection
}

//
// Create a VM snapshot and return its ID.
func (r *Client) CreateSnapshot(vmRef ref.Ref) (snapshot string, err error) {
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	snapsService := vmService.SnapshotsService()
	snap, err := snapsService.Add().Snapshot(
		ovirtsdk.NewSnapshotBuilder().
			Name(snapshotName).
			Description(snapshotDesc).
			PersistMemorystate(false).
			MustBuild(),
	).Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	snapshot = snap.MustSnapshot().MustId()
	return
}

//
// Remove all warm migration snapshots.
func (r *Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) (err error) {
	if len(precopies) == 0 {
		return
	}
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	snapsService := vmService.SnapshotsService()
	for i := range precopies {
		snapService := snapsService.SnapshotService(precopies[i].Snapshot)
		_, err = snapService.Remove().Async(true).Send()
		if err != nil {
			err = liberr.Wrap(err)
		}
	}
	return
}

//
// Create DataVolume checkpoints.
func (r *Client) CreateCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []*cdi.DataVolume) (checkpoints map[*cdi.DataVolume]cdi.DataVolumeCheckpoint, err error) {
	n := len(precopies)
	previous := precopies[n-2].Snapshot
	current := precopies[n-1].Snapshot

	checkpoints = make(map[*cdi.DataVolume]cdi.DataVolumeCheckpoint)
	for i := range datavolumes {
		dv := datavolumes[i]
		var currentDiskSnapshot, previousDiskSnapshot string
		currentDiskSnapshot, err = r.getDiskSnapshot(dv.Spec.Source.Imageio.DiskID, current)
		if err != nil {
			return
		}
		previousDiskSnapshot, err = r.getDiskSnapshot(dv.Spec.Source.Imageio.DiskID, previous)
		if err != nil {
			return
		}
		checkpoints[dv] = cdi.DataVolumeCheckpoint{
			Current:  currentDiskSnapshot,
			Previous: previousDiskSnapshot,
		}
	}
	return
}

//
// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state string, err error) {
	vm, _, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	status, _ := vm.Status()
	switch status {
	case ovirtsdk.VMSTATUS_DOWN, ovirtsdk.VMSTATUS_POWERING_DOWN:
		state = powerOff
	case ovirtsdk.VMSTATUS_UP, ovirtsdk.VMSTATUS_POWERING_UP:
		state = powerOn
	default:
		state = powerUnknown
	}
	return
}

//
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

//
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

//
// Determine whether the VM has been powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (poweredOff bool, err error) {
	powerState, err := r.PowerState(vmRef)
	if err != nil {
		return
	}
	poweredOff = powerState == powerOff
	return
}

//
// Close the connection to the oVirt API.
func (r *Client) Close() {
	if r.connection != nil {
		_ = r.connection.Close()
		r.connection = nil
	}
}

//
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
	vmResponse, err := vmService.Get().Send()
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

//
// Get the disk snapshot for this disk and this snapshot ID.
func (r *Client) getDiskSnapshot(diskID, targetSnapshotID string) (diskSnapshotID string, err error) {
	diskService := r.connection.SystemService().DisksService().DiskService(diskID)
	diskSnapshotsService := diskService.DiskSnapshotsService()
	diskSnapshotsListResponse, err := diskSnapshotsService.List().Send()
	if err != nil {
		err = liberr.Wrap(
			err,
			"Error listing snapshots on disk",
			"disk",
			diskID)
		return
	}

	diskSnapshotsList, success := diskSnapshotsListResponse.Snapshots()
	if !success || len(diskSnapshotsList.Slice()) == 0 {
		err = liberr.New(
			"No snapshots listed on disk",
			"disk",
			diskID)
		return
	}

	for _, diskSnapshot := range diskSnapshotsList.Slice() {
		if snapshot, success := diskSnapshot.Snapshot(); success {
			if snapshotID, success := snapshot.Id(); success && snapshotID == targetSnapshotID {
				if diskSnapshotID, success = diskSnapshot.Id(); success {
					return
				}
			}
		}
	}
	return
}

//
// Connect to the oVirt API.
func (r *Client) connect() (err error) {
	URL := r.Source.Provider.Spec.URL
	r.connection, err = ovirtsdk.NewConnectionBuilder().
		URL(URL).
		Username(r.user()).
		Password(r.password()).
		CACert(r.cacert()).
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
