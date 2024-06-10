package vsphere

import (
	"context"
	"fmt"
	liburl "net/url"
	"strconv"

	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/utils/ptr"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	snapshotName = "forklift-migration-precopy"
	snapshotDesc = "Forklift Operator warm migration precopy"
)

// vSphere VM Client
type Client struct {
	*plancontext.Context
	client *govmomi.Client
}

// Create a VM snapshot and return its ID.
func (r *Client) CreateSnapshot(vmRef ref.Ref) (id string, err error) {
	r.Log.V(1).Info("Creating snapshot", "vmRef", vmRef)
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	task, err := vm.CreateSnapshot(context.TODO(), snapshotName, snapshotDesc, false, true)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	res, err := task.WaitForResult(context.TODO(), nil)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	id = res.Result.(types.ManagedObjectReference).Value
	r.Log.Info("Created snapshot", "vmRef", vmRef, "id", id)

	return
}

// Check if a snapshot is ready to transfer.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, snapshot string) (ready bool, err error) {
	return true, nil
}

// Remove all warm migration snapshots.
func (r *Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) (err error) {

	r.Log.V(1).Info("RemoveSnapshot",
		"vmRef", vmRef,
		"precopies", precopies,
		"incremental", settings.Settings.VsphereIncrementalBackup)
	if len(precopies) == 0 {
		return
	}
	if settings.Settings.VsphereIncrementalBackup {
		// only necessary to clean up the last snapshot if this feature is enabled,
		// because all others will have already been cleaned up.
		lastSnapshot := precopies[len(precopies)-1].Snapshot
		err = r.removeSnapshot(vmRef, lastSnapshot, false)
	} else {
		rootSnapshot := precopies[0].Snapshot
		err = r.removeSnapshot(vmRef, rootSnapshot, true)
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

	r.Log.V(1).Info("SetCheckpoint",
		"vmRef", vmRef,
		"precopies", precopies,
		"datavolumes", datavolumes,
		"final", final,
		"current", current,
		"previous", previous)

	if settings.Settings.VsphereIncrementalBackup && previous != "" {
		var changeIds map[string]string
		changeIds, err = r.getChangeIds(vmRef, previous)
		if err != nil {
			return
		}
		for i := range datavolumes {
			dv := &datavolumes[i]
			dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, cdi.DataVolumeCheckpoint{
				Current:  current,
				Previous: changeIds[dv.Spec.Source.VDDK.BackingFile],
			})
			dv.Spec.FinalCheckpoint = final
		}
		err = r.removeSnapshot(vmRef, previous, false)
		if err != nil {
			return
		}
	} else {
		for i := range datavolumes {
			dv := &datavolumes[i]
			dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, cdi.DataVolumeCheckpoint{
				Current:  current,
				Previous: previous,
			})
			dv.Spec.FinalCheckpoint = final
		}
	}
	return
}

// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state planapi.VMPowerState, err error) {
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
		state = planapi.VMPowerStateOn
	case types.VirtualMachinePowerStatePoweredOff:
		state = planapi.VMPowerStateOff
	default:
		state = planapi.VMPowerStateUnknown
	}
	return
}

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

// Close the connection to the vSphere API.
func (r *Client) Close() {
	if r.client != nil {
		_ = r.client.Logout(context.TODO())
		r.client.CloseIdleConnections()
		r.client = nil
	}
}

func (c *Client) Finalize(vms []*planapi.VMStatus, planName string) {
}

func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	ready = true
	return
}

// Get the changeId for a VM snapshot.
func (r *Client) getChangeIds(vmRef ref.Ref, snapshotId string) (changeIdMapping map[string]string, err error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}

	var snapshot mo.VirtualMachineSnapshot
	err = vm.Properties(
		context.TODO(),
		types.ManagedObjectReference{Type: "VirtualMachineSnapshot", Value: snapshotId},
		[]string{"config.hardware.device"},
		&snapshot)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Reference().Value, "snapshot", snapshotId)
		return
	}

	changeIdMapping = make(map[string]string)
	for _, device := range snapshot.Config.Hardware.Device {
		vDevice := device.GetVirtualDevice()
		switch dev := vDevice.Backing.(type) {
		case *types.VirtualDiskFlatVer2BackingInfo:
			changeIdMapping[trimBackingFileName(dev.FileName)] = dev.ChangeId
		case *types.VirtualDiskSparseVer2BackingInfo:
			changeIdMapping[trimBackingFileName(dev.FileName)] = dev.ChangeId
		case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
			changeIdMapping[trimBackingFileName(dev.FileName)] = dev.ChangeId
		case *types.VirtualDiskRawDiskVer2BackingInfo:
			changeIdMapping[trimBackingFileName(dev.DescriptorFileName)] = dev.ChangeId
		}

	}

	return
}

// Get the VM by ref.
func (r *Client) getVM(vmRef ref.Ref) (vsphereVm *object.VirtualMachine, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	searchIndex := object.NewSearchIndex(r.client.Client)
	vsphereRef, err := searchIndex.FindByUuid(context.TODO(), nil, vm.UUID, true, ptr.To(false))
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

// Remove a VM snapshot and optionally its children.
func (r *Client) removeSnapshot(vmRef ref.Ref, snapshot string, children bool) (err error) {
	r.Log.Info("Removing snapshot",
		"vmRef", vmRef,
		"snapshot", snapshot,
		"children", children)

	vm, err := r.getVM(vmRef)
	if err != nil {
		return
	}
	_, err = vm.RemoveSnapshot(context.TODO(), snapshot, children, nil)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

// Connect to the vSphere API.
func (r *Client) connect() error {
	r.Close()
	url, err := liburl.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		return liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(r.user(), r.password())
	soapClient := soap.NewClient(url, r.getInsecureSkipVerifyFlag())
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
	return r.Source.Provider.Status.Fingerprint
}

// getInsecureSkipVerifyFlag gets the insecureSkipVerify boolean flag
// value from the provider connection secret.
func (r *Client) getInsecureSkipVerifyFlag() bool {
	insecure, found := r.Source.Secret.Data["insecureSkipVerify"]
	if !found {
		return false
	}

	insecureSkipVerify, err := strconv.ParseBool(string(insecure))
	if err != nil {
		return false
	}

	return insecureSkipVerify
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	// no-op
	return
}
