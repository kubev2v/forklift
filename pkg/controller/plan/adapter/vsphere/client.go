package vsphere

import (
	"context"
	"fmt"
	liburl "net/url"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	snapshotName = "forklift-migration-precopy"
	snapshotDesc = "Forklift Operator warm migration precopy"
	taskType     = "Task"
)

// vSphere VM Client
type Client struct {
	*plancontext.Context
	client      *govmomi.Client
	hostClients map[string]*govmomi.Client
}

// Create a VM snapshot and return its ID.
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (snapshotId string, creationTaskId string, err error) {
	r.Log.V(1).Info("Creating snapshot", "vmRef", vmRef)
	vm, err := r.getVM(vmRef, hostsFunc)
	if err != nil {
		return
	}
	task, err := vm.CreateSnapshot(context.TODO(), snapshotName, snapshotDesc, false, true)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return "", task.Reference().Value, nil
}

// Remove a VM snapshot.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hosts util.HostsFunc) (taskId string, err error) {
	r.Log.V(1).Info("RemoveSnapshot",
		"vmRef", vmRef,
		"snapshot", snapshot)
	taskId, err = r.removeSnapshot(vmRef, snapshot, false, hosts)
	return
}

// Set DataVolume checkpoints.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hosts util.HostsFunc) (err error) {
	n := len(precopies)
	var previous planapi.Precopy
	current := precopies[n-1]
	if n >= 2 {
		previous = precopies[n-2]
	}

	r.Log.V(1).Info("SetCheckpoint",
		"vmRef", vmRef,
		"precopies", precopies,
		"datavolumes", datavolumes,
		"final", final,
		"current", current.Snapshot,
		"previous", previous.Snapshot)

	changeIds := previous.DeltaMap()
	for i := range datavolumes {
		dv := &datavolumes[i]
		dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, cdi.DataVolumeCheckpoint{
			Current:  current.Snapshot,
			Previous: changeIds[dv.Spec.Source.VDDK.BackingFile],
		})
		dv.Spec.FinalCheckpoint = final
	}
	return
}

// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state planapi.VMPowerState, err error) {
	vm, err := r.getVM(vmRef, nullableHosts)
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
	vm, err := r.getVM(vmRef, nullableHosts)
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
	vm, err := r.getVM(vmRef, nullableHosts)
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
	vm, err := r.getVM(vmRef, nullableHosts)
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
	for _, client := range r.hostClients {
		_ = client.Logout(context.TODO())
		client.CloseIdleConnections()
	}
	r.hostClients = nil
}

func (c *Client) Finalize(vms []*planapi.VMStatus, planName string) {
}

func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	ready = true
	return
}

// Get a mapping of disks and change IDs for a given snapshot.
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshotId string, hosts util.HostsFunc) (changeIdMapping map[string]string, err error) {
	vm, err := r.getVM(vmRef, hosts)
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

	r.Log.V(1).Info("GetSnapshotDeltas",
		"vmRef", vmRef,
		"snapshot", snapshotId,
		"deltas", changeIdMapping)

	return
}

// Check if a snapshot is removed
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	r.Log.Info("Check Snapshot Remove", "vmRef", vmRef, "precopy", precopy)
	taskInfo, err := r.getTaskById(vmRef, precopy.RemoveTaskId, hosts)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	return r.checkTaskStatus(taskInfo)
}

// Check if a snapshot is ready to transfer.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, snapshotId string, err error) {
	r.Log.Info("Check Snapshot Ready", "vmRef", vmRef, "precopy", precopy)
	taskInfo, err := r.getTaskById(vmRef, precopy.CreateTaskId, hosts)
	if err != nil {
		return false, "", liberr.Wrap(err)
	}
	ready, err = r.checkTaskStatus(taskInfo)
	if err != nil {
		return false, "", liberr.Wrap(err)
	}
	if !ready {
		// Task is not finished retry
		return false, "", nil
	}
	if taskInfo.Result == nil {
		// Empty result so the task did not finish retry
		return false, "", nil
	}
	snapshotId = taskInfo.Result.(types.ManagedObjectReference).Value
	return
}

func (r *Client) checkTaskStatus(taskInfo *types.TaskInfo) (ready bool, err error) {
	r.Log.Info("Snapshot task", "task", taskInfo.Task.Value, "name", taskInfo.Name, "status", taskInfo.State)
	switch taskInfo.State {
	case types.TaskInfoStateSuccess:
		return true, nil
	case types.TaskInfoStateError:
		return false, fmt.Errorf(taskInfo.Error.LocalizedMessage)
	default:
		return false, nil
	}
}

func (r *Client) getClientFromVmRef(vmRef ref.Ref, hosts util.HostsFunc) (client *vim25.Client, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}
	return r.getClient(vm, hosts)
}

func (r *Client) getTaskById(vmRef ref.Ref, taskId string, hosts util.HostsFunc) (*types.TaskInfo, error) {
	r.Log.V(1).Info("Get task by id", "taskId", taskId, "vmRef", vmRef)

	// Get the ESXi client for the haTasks
	client, err := r.getClientFromVmRef(vmRef, hosts)
	if err != nil {
		return nil, err
	}
	// Create a collector to receive the tasks
	pc := property.DefaultCollector(client)
	pc, err = pc.Create(context.TODO())
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer pc.Destroy(context.TODO())

	// Retrieve the task from ESXi host
	taskRef := types.ManagedObjectReference{
		Type:  taskType,
		Value: taskId,
	}
	var content []types.ObjectContent
	err = pc.RetrieveOne(context.TODO(), taskRef, []string{"info"}, &content)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("task %s not found", taskId)
	}
	if len(content[0].PropSet) == 0 {
		return nil, fmt.Errorf("task %s not found property set", taskId)
	}
	if content[0].PropSet[0].Val == nil {
		return nil, fmt.Errorf("no task value found for task %s", taskId)
	}
	task := content[0].PropSet[0].Val.(types.TaskInfo)
	return &task, nil
}

func (r *Client) getClient(vm *model.VM, hosts util.HostsFunc) (client *vim25.Client, err error) {
	// if coldLocal, vErr := r.Plan.VSphereColdLocal(); vErr == nil && coldLocal {
	// 	// when virt-v2v runs the migration, forklift-controller should interact only
	// 	// with the component that serves the SDK endpoint of the provider
	// 	client = r.client.Client
	// 	return
	// }
	if useV2vForTransfer, vErr := r.Plan.ShouldUseV2vForTransfer(); vErr == nil && useV2vForTransfer {
		// when virt-v2v runs the migration, forklift-controller should interact only
		// with the component that serves the SDK endpoint of the provider
		client = r.client.Client
		return
	}

	if r.Source.Provider.Spec.Settings[v1beta1.SDK] == v1beta1.ESXI {
		// when migrating from ESXi host, we use the client of the SDK endpoint of the provider,
		// there's no need in a different client (the ESXi host is the only component involved in the migration)
		client = r.client.Client
		return
	}

	host := &model.Host{}
	if err = r.Source.Inventory.Get(host, vm.Host); err != nil {
		err = liberr.Wrap(err, "host", vm.Host)
		return
	}

	if cachedClient, found := r.hostClients[host.ID]; found {
		// return the cached client for the ESXi host
		client = cachedClient.Client
		return
	}

	if hostMap, hostsErr := hosts(); hostsErr == nil {
		if hostDef, found := hostMap[host.ID]; found {
			// create a new client for the ESXi host we are going to transfer the disk(s) from, and cache it
			client, err = r.getHostClient(hostDef, host)
		} else {
			// there is no network defined for the ESXi host, so we will transfer the disk(s) from vCenter and
			// thus there is no need in a client for the ESXi host but we use the client for vCenter instead
			client = r.client.Client
		}
	} else {
		err = liberr.Wrap(hostsErr)
	}
	return
}

func (r *Client) getHostClient(hostDef *v1beta1.Host, host *model.Host) (client *vim25.Client, err error) {
	url, err := liburl.Parse("https://" + hostDef.Spec.IpAddress + "/sdk")
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	ref := hostDef.Spec.Secret
	secret := &core.Secret{}
	err = r.Get(
		context.TODO(),
		k8sclient.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	url.User = liburl.UserPassword(string(secret.Data["user"]), string(secret.Data["password"]))
	soapClient := soap.NewClient(url, base.GetInsecureSkipVerifyFlag(r.Source.Secret))
	soapClient.SetThumbprint(url.Host, host.Thumbprint)
	vimClient, err := vim25.NewClient(context.TODO(), soapClient)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	hostClient := &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	if err = hostClient.Login(context.TODO(), url.User); err != nil {
		err = liberr.Wrap(err)
		return
	}

	if r.hostClients == nil {
		r.hostClients = make(map[string]*govmomi.Client)
	}
	r.hostClients[host.ID] = hostClient
	client = hostClient.Client
	return
}

// Get the VM by ref.
func (r *Client) getVM(vmRef ref.Ref, hosts util.HostsFunc) (vsphereVm *object.VirtualMachine, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	client, err := r.getClient(vm, hosts)
	if err != nil {
		return
	}

	searchIndex := object.NewSearchIndex(client)
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
	vsphereVm = object.NewVirtualMachine(client, vsphereRef.Reference())
	return
}

func nullableHosts() (hosts map[string]*v1beta1.Host, err error) {
	return
}

// Remove a VM snapshot and optionally its children.
func (r *Client) removeSnapshot(vmRef ref.Ref, snapshot string, children bool, hosts util.HostsFunc) (taskId string, err error) {
	r.Log.Info("Removing snapshot",
		"vmRef", vmRef,
		"snapshot", snapshot,
		"children", children)

	vm, err := r.getVM(vmRef, hosts)
	if err != nil {
		return
	}
	task, err := vm.RemoveSnapshot(context.TODO(), snapshot, children, nil)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	return task.Reference().Value, nil
}

// Connect to the vSphere API.
func (r *Client) connect() error {
	r.Close()
	url, err := liburl.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		return liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(r.user(), r.password())
	soapClient := soap.NewClient(url, base.GetInsecureSkipVerifyFlag(r.Source.Secret))
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

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	// no-op
	return
}
