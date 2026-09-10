package vsphere

import (
	"context"
	"errors"
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
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
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
	snapshotName           = "forklift-migration-precopy"
	snapshotDesc           = "Forklift Operator warm migration precopy"
	taskType               = "Task"
	createSnapshotTaskName = "vim.VirtualMachine.createSnapshot"
	removeSnapshotTaskName = "vim.vm.Snapshot.remove"
)

var ErrTaskNotFound = errors.New("not found")
var ErrTaskNotFoundPropSet = errors.New("not found property set")
var ErrTaskValueNotFound = errors.New("no task value found for task")

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

	// Check if there's already a running CreateSnapshot task - prevents duplicates
	if existingTaskId := r.findRunningSnapshotTask(vmRef, vm, hostsFunc, createSnapshotTaskName); existingTaskId != "" {
		return "", existingTaskId, nil
	}

	task, err := vm.CreateSnapshot(context.TODO(), snapshotName, snapshotDesc, false, true)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	creationTaskId = task.Reference().Value
	return "", creationTaskId, nil
}

// Check if there's already a running CreateSnapshot task on the VM to prevent duplicates
func (r *Client) findRunningSnapshotTask(vmRef ref.Ref, vm *object.VirtualMachine, hosts util.HostsFunc, snapshotTaskName string) string {
	// Get the ESXi client
	client, err := r.getClientFromVmRef(vmRef, hosts)
	if err != nil {
		return ""
	}

	// Create property collector
	pc := property.DefaultCollector(client)
	pc, err = pc.Create(context.TODO())
	if err != nil {
		return ""
	}
	//nolint:errcheck
	defer pc.Destroy(context.TODO())

	// Get VM recent tasks
	var vmObj mo.VirtualMachine
	err = pc.RetrieveOne(context.TODO(), vm.Reference(), []string{"recentTask"}, &vmObj)
	if err != nil {
		return ""
	}

	// Check for running CreateSnapshot tasks
	for _, taskRef := range vmObj.RecentTask {
		var task mo.Task
		err = pc.RetrieveOne(context.TODO(), taskRef, []string{"info"}, &task)
		if err != nil {
			continue
		}
		// Check if this is a running CreateSnapshot task
		if task.Info.Name == snapshotTaskName &&
			(task.Info.State == types.TaskInfoStateRunning || task.Info.State == types.TaskInfoStateQueued) {
			return taskRef.Value
		}
	}

	return ""
}

// Remove a VM snapshot.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hosts util.HostsFunc) (taskId string, err error) {
	r.Log.V(1).Info("RemoveSnapshot",
		"vmRef", vmRef,
		"snapshot", snapshot)
	vm, err := r.getVM(vmRef, hosts)
	if err != nil {
		return
	}
	// Check if there's already a running remove snapshot task - prevents duplicates
	if existingTaskId := r.findRunningSnapshotTask(vmRef, vm, hosts, removeSnapshotTaskName); existingTaskId != "" {
		return existingTaskId, nil
	}
	r.Log.Info("Removing snapshot",
		"vmRef", vmRef,
		"snapshot", snapshot,
		"children", false)

	task, err := vm.RemoveSnapshot(context.TODO(), snapshot, false, ptr.To(true))
	if err != nil {
		return "", liberr.Wrap(err)
	}
	return task.Reference().Value, nil
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
		alreadyExists := false
		for _, checkpoint := range dv.Spec.Checkpoints {
			if checkpoint.Current == current.Snapshot {
				r.Log.V(1).Info("Snapshot already exists in DataVolume checkpoints list", "vmRef", vmRef, "DataVolume", dv.Name, "checkpoints", dv.Spec.Checkpoints)
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, cdi.DataVolumeCheckpoint{
				Current:  current.Snapshot,
				Previous: changeIds[dv.Spec.Source.VDDK.BackingFile],
			})
		}
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

// Finalize is called in a goroutine after all VMs in the plan have completed.
// If tagging is enabled, it attaches a vSphere tag to every successfully migrated source VM.
// All errors are logged only; they must not affect the already-completed migration.
func (c *Client) Finalize(vms []*planapi.VMStatus, planName string) {
	defer func() {
		if r := recover(); r != nil {
			c.Log.Info("Recovered from panic in Finalize", "err", r)
		}
	}()

	// Skip for standalone ESXi — tagging requires vCenter.
	if c.Source.Provider.Spec.Settings[v1beta1.SDK] == v1beta1.ESXI {
		c.Log.V(1).Info("Skipping post-migration tagging for standalone ESXi provider")
		return
	}

	// Early filter — before opening any connection, check if there's work to do.
	var succeeded []*planapi.VMStatus
	for _, vm := range vms {
		if vm.HasCondition(v1beta1.ConditionSucceeded) {
			succeeded = append(succeeded, vm)
		}
	}

	if len(succeeded) == 0 {
		c.Log.V(1).Info("No VMs succeeded — skipping post-migration tagging")
		return
	}

	if !settings.Settings.PostMigrationTaggingEnabled {
		c.Log.V(1).Info("Post-migration tagging is disabled")
		return
	}

	// Open a fresh REST session local to this goroutine.
	restClient, err := c.connectREST()
	if err != nil {
		c.Log.Error(err, "Failed to connect to vSphere REST API for post-migration tagging")
		return
	}
	defer func() { _ = restClient.Logout(context.TODO()) }()

	c.finalizeTagMigrated(context.TODO(), restClient, succeeded)
}

// connectREST opens a fresh vSphere VAPI REST session using provider credentials.
// Returns a REST client owned by the caller. Does not mutate c.client.
func (c *Client) connectREST() (*rest.Client, error) {
	url, err := liburl.Parse(c.Source.Provider.Spec.URL)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(c.user(), c.password())
	soapClient := soap.NewClient(url, base.GetInsecureSkipVerifyFlag(c.Source.Secret))
	soapClient.SetThumbprint(url.Host, c.thumbprint())
	vimClient, err := vim25.NewClient(context.TODO(), soapClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	restClient := rest.NewClient(vimClient)
	if err := restClient.Login(context.TODO(), url.User); err != nil {
		return nil, liberr.Wrap(err)
	}
	return restClient, nil
}

// finalizeTagMigrated idempotently ensures the configured category and tag exist,
// then attaches the tag to all succeeded VMs in a batch API call.
// Assumes vms contains only succeeded VMs (filtered by Finalize).
func (c *Client) finalizeTagMigrated(ctx context.Context, restClient *rest.Client, vms []*planapi.VMStatus) {
	if len(vms) == 0 {
		return
	}

	tagManager := tags.NewManager(restClient)
	tagID, err := c.ensureTag(ctx, tagManager)
	if err != nil {
		c.Log.Error(err, "Failed to ensure post-migration tag exists")
		return
	}

	// Build the list of VM morefs.
	var refs []types.ManagedObjectReference
	for _, vm := range vms {
		refs = append(refs, types.ManagedObjectReference{
			Type:  "VirtualMachine",
			Value: vm.ID,
		})
	}

	// Batch-attach the tag. This is idempotent: if a VM is already tagged, it's a no-op.
	// AttachTagToMultipleObjects was added in vSphere 6.5.
	err = tagManager.AttachTagToMultipleObjects(ctx, tagID, refsToMoReferences(refs))
	if err != nil {
		c.Log.Error(err, "Failed to batch-attach post-migration tag", "count", len(refs))
		return
	}

	c.Log.Info("Attached post-migration tag to migrated VMs",
		"category", settings.Settings.PostMigrationTagCategory,
		"tag", settings.Settings.PostMigrationTagName,
		"count", len(refs))
}

// ensureTag guarantees the configured category and tag exist in vSphere and
// returns the tag URN. Creates category/tag if absent.
// Safe to call concurrently — handles create races via list-after-create-failure.
func (c *Client) ensureTag(ctx context.Context, tagManager *tags.Manager) (string, error) {
	categoryName := settings.Settings.PostMigrationTagCategory
	tagName := settings.Settings.PostMigrationTagName

	categoryID, err := c.resolveOrCreateTagCategory(ctx, tagManager, categoryName)
	if err != nil {
		return "", err
	}

	return c.resolveOrCreateTag(ctx, tagManager, categoryID, tagName)
}

// resolveOrCreateTagCategory ensures the tag category exists and returns its ID.
// Safe to call concurrently — handles create races via get-after-create-failure.
func (c *Client) resolveOrCreateTagCategory(ctx context.Context, tagManager *tags.Manager, categoryName string) (string, error) {
	cat, err := tagManager.GetCategory(ctx, categoryName)
	if err != nil {
		if !rest.IsStatusError(err, 404) {
			return "", liberr.Wrap(err, "category", categoryName)
		}
		// Category doesn't exist — create it.
		categoryID, err := tagManager.CreateCategory(ctx, &tags.Category{
			Name:            categoryName,
			Cardinality:     "MULTIPLE",
			AssociableTypes: []string{"VirtualMachine"},
		})
		if err != nil {
			// Create can fail with 400 if another caller created it concurrently.
			cat, getErr := tagManager.GetCategory(ctx, categoryName)
			if getErr != nil {
				return "", liberr.Wrap(err, "category", categoryName)
			}
			return cat.ID, nil
		}
		return categoryID, nil
	}
	return cat.ID, nil
}

// resolveOrCreateTag ensures the tag exists in the category and returns its ID.
// Safe to call concurrently — handles create races via list-after-create-failure.
func (c *Client) resolveOrCreateTag(ctx context.Context, tagManager *tags.Manager, categoryID, tagName string) (string, error) {
	// Only create after successful list confirms absence.
	existingTags, err := tagManager.GetTagsForCategory(ctx, categoryID)
	if err != nil {
		// List failure is fatal — cannot determine if tag exists.
		return "", liberr.Wrap(err, "category", categoryID)
	}
	for _, t := range existingTags {
		if t.Name == tagName {
			return t.ID, nil
		}
	}

	// Tag confirmed absent — create it.
	tagID, err := tagManager.CreateTag(ctx, &tags.Tag{
		Name:       tagName,
		CategoryID: categoryID,
	})
	if err != nil {
		// Create can fail with 400 if another caller created it concurrently.
		existingTags, getErr := tagManager.GetTagsForCategory(ctx, categoryID)
		if getErr != nil {
			return "", liberr.Wrap(err, "tag", tagName)
		}
		for _, t := range existingTags {
			if t.Name == tagName {
				return t.ID, nil
			}
		}
		return "", liberr.Wrap(err, "tag", tagName)
	}

	return tagID, nil
}

// refsToMoReferences converts []ManagedObjectReference to []mo.Reference.
// ManagedObjectReference implements mo.Reference via its Reference() method.
func refsToMoReferences(refs []types.ManagedObjectReference) []mo.Reference {
	result := make([]mo.Reference, len(refs))
	for i := range refs {
		result[i] = refs[i]
	}
	return result
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
	if err == nil {
		return r.checkTaskStatus(taskInfo)
	}

	notFound := errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrTaskNotFoundPropSet) || errors.Is(err, ErrTaskValueNotFound)
	alreadyDeleted := fault.Is(err, &types.ManagedObjectNotFound{})
	if !notFound && !alreadyDeleted {
		return false, liberr.Wrap(err)
	}

	// If the task is done and gone, make sure the snapshot itself is gone
	r.Log.Info("Snapshot removal task not found, checking for existing snapshot", "vmRef", vmRef, "precopy", precopy)
	vm := &model.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return false, liberr.Wrap(err)
	}
	if vm.Snapshot.ID == "" {
		return true, nil
	}

	return false, nil
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
	switch taskInfo.State {
	case types.TaskInfoStateSuccess:
		return true, nil
	case types.TaskInfoStateError:
		return false, fmt.Errorf("error cheking task status: %s", taskInfo.Error.LocalizedMessage)
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
		return nil, fmt.Errorf("task %s %w", taskId, ErrTaskNotFound)
	}
	if len(content[0].PropSet) == 0 {
		return nil, fmt.Errorf("task %s %w", taskId, ErrTaskNotFoundPropSet)
	}
	if content[0].PropSet[0].Val == nil {
		return nil, fmt.Errorf("%w %s", ErrTaskValueNotFound, taskId)
	}
	task := content[0].PropSet[0].Val.(types.TaskInfo)
	return &task, nil
}

func (r *Client) getClient(vm *model.VM, hosts util.HostsFunc) (client *vim25.Client, err error) {
	if useV2vForTransfer, vErr := r.Plan.ShouldUseV2vForTransfer(ref.Ref{ID: vm.ID}); vErr == nil && useV2vForTransfer {
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
	url, err := liburl.Parse("https://" + formatHostAddress(hostDef.Spec.IpAddress) + "/sdk")
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
	uuid := vm.InstanceUUID
	useInstanceUUID := uuid != ""
	if !useInstanceUUID {
		uuid = vm.UUID
	}
	vsphereRef, err := searchIndex.FindByUuid(context.TODO(), nil, uuid, true, ptr.To(useInstanceUUID))
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

// getDiskBacking returns vSphere disk backing info (VVol/RDM/VMDK) for a named disk file.
// Uses RetrieveOne on the VM moref directly — no datacenter search needed since the controller
// always has managed object reference IDs from inventory.
func (r *Client) getDiskBacking(ctx context.Context, vmId, diskFile string) (*resolver.DiskBacking, error) {
	if r.client == nil {
		if err := r.connect(); err != nil {
			return nil, liberr.Wrap(err)
		}
	}

	var vmMo mo.VirtualMachine
	err := property.DefaultCollector(r.client.Client).RetrieveOne(
		ctx,
		types.ManagedObjectReference{Type: "VirtualMachine", Value: vmId},
		[]string{"config.hardware.device"},
		&vmMo,
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return resolver.DiskBackingFromDevices(vmMo.Config.Hardware.Device, diskFile)
}

func (r *Client) getNAAFromDatastore(ctx context.Context, datastoreRef ref.Ref) (string, error) {
	ds := &model.Datastore{}
	err := r.Source.Inventory.Find(ds, datastoreRef)
	if err != nil {
		return "", liberr.Wrap(err, "datastore", datastoreRef.String())
	}

	if r.client == nil {
		if err := r.connect(); err != nil {
			return "", liberr.Wrap(err, "failed to connect to vSphere")
		}
	}

	properties := []string{"info"}
	var dsMo mo.Datastore
	err = property.DefaultCollector(r.client.Client).RetrieveOne(
		ctx,
		types.ManagedObjectReference{
			Type:  "Datastore",
			Value: datastoreRef.ID,
		},
		properties,
		&dsMo,
	)
	if err != nil {
		return "", liberr.Wrap(err, "failed to retrieve datastore properties")
	}

	dsinfo := dsMo.Info
	vmfsInfo, ok := dsinfo.(*types.VmfsDatastoreInfo)
	if !ok {
		return "", liberr.New(
			fmt.Sprintf("datastore '%s' is not a VMFS datastore (Type: %T)", ds.Name, dsMo.Info.GetDatastoreInfo()),
		)
	}

	if len(vmfsInfo.Vmfs.Extent) == 0 {
		return "", liberr.New(
			fmt.Sprintf("VMFS datastore '%s' has no associated extents/devices", ds.Name),
		)
	}

	// The DiskName field contains the NAA ID (e.g., "naa.600508b1001c1234567890abcdef1234")
	naaID := vmfsInfo.Vmfs.Extent[0].DiskName

	r.Log.Info("Retrieved NAA ID for datastore",
		"datastore", ds.Name,
		"naaID", naaID,
		"vmfsVersion", vmfsInfo.Vmfs.Version,
		"vmfsUUID", vmfsInfo.Vmfs.Uuid,
		"extentCount", len(vmfsInfo.Vmfs.Extent))

	return naaID, nil
}
