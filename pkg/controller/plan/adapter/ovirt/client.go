package ovirt

import (
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

const (
	snapshotName = "forklift-migration-precopy"
	snapshotDesc = "Forklift Operator warm migration precopy"
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
// Remove a VM snapshot.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, _ bool) (err error) {
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	snapsService := vmService.SnapshotsService()
	snapService := snapsService.SnapshotService(snapshot)
	_, err = snapService.Remove().Async(true).Send()
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state plan.VMPowerState, err error) {
	vm, _, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	status, _ := vm.Status()
	switch status {
	case ovirtsdk.VMSTATUS_DOWN, ovirtsdk.VMSTATUS_POWERING_DOWN:
		state = plan.VMPowerStateOff
	case ovirtsdk.VMSTATUS_UP, ovirtsdk.VMSTATUS_POWERING_UP:
		state = plan.VMPowerStateOn
	default:
		state = plan.VMPowerStateUnknown
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
		_, err = vmService.Stop().Send()
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
	poweredOff = powerState == plan.VMPowerStateOff
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
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmRef.String(),
				err.Error()))
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
