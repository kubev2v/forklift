package vsphere

import (
	"context"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	liburl "net/url"
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
// vSphere VM Client
type Client struct {
	*plancontext.Context
	client *govmomi.Client
}

//
// Create a VM snapshot and return its ID.
func (r *Client) CreateSnapshot(vmRef ref.Ref) (snapshot string, err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	task, err := vm.CreateSnapshot(context.TODO(), snapshotName, snapshotDesc, false, true)
	res, err := task.WaitForResult(context.TODO(), nil)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	snapshot = res.Result.(types.ManagedObjectReference).Value
	return
}

//
// Remove a VM snapshot.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, all bool) (err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	_, err = vm.RemoveSnapshot(context.TODO(), snapshot, all, nil)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

//
// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state string, err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	powerState, err := vm.PowerState(context.TODO())
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch powerState {
	case types.VirtualMachinePowerStatePoweredOn:
		state = powerOn
	case types.VirtualMachinePowerStatePoweredOff:
		state = powerOff
	default:
		state = powerUnknown
	}
	return
}

//
// Power on the VM.
func (r *Client) PowerOn(vmRef ref.Ref) (err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	powerState, err := vm.PowerState(context.TODO())
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if powerState != types.VirtualMachinePowerStatePoweredOn {
		_, err = vm.PowerOn(context.TODO())
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	return
}

//
// Power off the VM. Requires guest tools to be installed.
func (r *Client) PowerOff(vmRef ref.Ref) (err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	powerState, err := vm.PowerState(context.TODO())
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if powerState == types.VirtualMachinePowerStatePoweredOff {
		return nil
	}
	err = vm.ShutdownGuest(context.TODO())
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

//
// Determine whether the VM has been powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (poweredOff bool, err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	powerState, err := vm.PowerState(context.TODO())
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	poweredOff = powerState == types.VirtualMachinePowerStatePoweredOff
	return
}

//
// Close the connection to the vSphere API.
func (r *Client) Close() {
	if r.client != nil {
		_ = r.client.Logout(context.TODO())
		r.client.CloseIdleConnections()
		r.client = nil
	}
}

//
// Get the VM by ref.
func (r *Client) getVM(vmRef ref.Ref) (vsphereVm *object.VirtualMachine, err error) {
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

	searchIndex := object.NewSearchIndex(r.client.Client)
	instanceUUID := false
	vsphereRef, err := searchIndex.FindByUuid(context.TODO(), nil, vm.UUID, true, &instanceUUID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if vsphereRef == nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s source lookup failed",
				vmRef.String()))
		return
	}
	vsphereVm = object.NewVirtualMachine(r.client.Client, vsphereRef.Reference())
	return
}

//
// Connect to the vSphere API.
func (r *Client) connect() error {
	r.Close()
	url, err := liburl.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		return liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(
		r.user(),
		r.password())
	soapClient := soap.NewClient(url, false)
	soapClient.SetThumbprint(url.Host, r.thumbprint())
	vimClient, err := vim25.NewClient(context.TODO(), soapClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.client = &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	err = r.client.Login(context.TODO(), url.User)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
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

func (r *Client) thumbprint() string {
	if thumbprint, found := r.Source.Secret.Data["thumbprint"]; found {
		return string(thumbprint)
	}
	return ""
}
