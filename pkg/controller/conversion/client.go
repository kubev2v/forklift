package conversion

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	snapshotName           = "forklift-deep-inspection"
	snapshotDesc           = "Forklift Operator deep inspection"
	taskType               = "Task"
	createSnapshotTaskName = "vim.VirtualMachine.createSnapshot"
	removeSnapshotTaskName = "vim.vm.Snapshot.remove"
)

// Client performs vSphere snapshot operations for a single VM using one vSphere session.
type Client struct {
	client   *govmomi.Client
	Log      logging.LevelLogger
	Provider *api.Provider
	VMRef    ref.Ref
}

// NewSnapshotClient creates a snapshot client for the given VM and govmomi session.
func NewSnapshotClient(log logging.LevelLogger, client *govmomi.Client, provider *api.Provider, vmRef ref.Ref) (*Client, error) {
	if client == nil {
		return nil, liberr.New("govmomi Client is required")
	}
	if provider == nil {
		return nil, liberr.New("Provider is required")
	}
	if vmRef.ID == "" {
		return nil, liberr.New("vmRef.ID (vSphere VirtualMachine managed object id) is required")
	}
	if log == nil {
		log = logging.WithName("deep-inspection-snapshot")
	}
	return &Client{
		client:   client,
		Log:      log,
		Provider: provider,
		VMRef:    vmRef,
	}, nil
}

func (r *Client) vim() *vim25.Client {
	return r.client.Client
}

// vmObject returns the VirtualMachine handle for the configured VM MoRef.
func (r *Client) vmObject() *object.VirtualMachine {
	moRef := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: r.VMRef.ID,
	}
	return object.NewVirtualMachine(r.vim(), moRef)
}

// CreateSnapshot creates a VM snapshot and returns the vSphere create task id.
func (r *Client) CreateSnapshot() (snapshotID string, creationTaskID string, err error) {
	r.Log.V(1).Info("Creating snapshot", "vmRef", r.VMRef)
	vm := r.vmObject()

	if existingTaskID := r.findRunningSnapshotTask(vm, createSnapshotTaskName); existingTaskID != "" {
		return "", existingTaskID, nil
	}

	task, err := vm.CreateSnapshot(context.TODO(), snapshotName, snapshotDesc, false, true)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	creationTaskID = task.Reference().Value
	return "", creationTaskID, nil
}

func (r *Client) findRunningSnapshotTask(vm *object.VirtualMachine, snapshotTaskName string) string {
	client := r.vim()

	pc := property.DefaultCollector(client)
	pc, err := pc.Create(context.TODO())
	if err != nil {
		return ""
	}
	//nolint:errcheck
	defer pc.Destroy(context.TODO())

	var vmObj mo.VirtualMachine
	err = pc.RetrieveOne(context.TODO(), vm.Reference(), []string{"recentTask"}, &vmObj)
	if err != nil {
		return ""
	}

	for _, taskRef := range vmObj.RecentTask {
		var task mo.Task
		err = pc.RetrieveOne(context.TODO(), taskRef, []string{"info"}, &task)
		if err != nil {
			continue
		}
		if task.Info.Name == snapshotTaskName &&
			(task.Info.State == types.TaskInfoStateRunning || task.Info.State == types.TaskInfoStateQueued) {
			return taskRef.Value
		}
	}

	return ""
}

// RemoveSnapshot removes a snapshot by MoRef value; returns the remove task id.
func (r *Client) RemoveSnapshot(snapshot string) (taskID string, err error) {
	r.Log.V(1).Info("RemoveSnapshot",
		"vmRef", r.VMRef,
		"snapshot", snapshot)
	vm := r.vmObject()
	if existingTaskID := r.findRunningSnapshotTask(vm, removeSnapshotTaskName); existingTaskID != "" {
		return existingTaskID, nil
	}
	r.Log.Info("Removing snapshot",
		"vmRef", r.VMRef,
		"snapshot", snapshot,
		"children", false)

	task, err := vm.RemoveSnapshot(context.TODO(), snapshot, false, nil)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	return task.Reference().Value, nil
}

// CheckCreateTaskReady waits for a snapshot create task to finish and returns the snapshot MoRef.
func (r *Client) CheckCreateTaskReady(createTaskID string) (ready bool, snapshotID string, err error) {
	r.Log.Info("Check snapshot create task", "vmRef", r.VMRef, "taskId", createTaskID)
	taskInfo, err := r.getTaskByID(createTaskID)
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
	snapshotID = taskInfo.Result.(types.ManagedObjectReference).Value
	return true, snapshotID, nil
}

// CheckRemoveTaskReady waits for a snapshot remove task to finish.
func (r *Client) CheckRemoveTaskReady(removeTaskID string) (ready bool, err error) {
	r.Log.Info("Check snapshot remove task", "vmRef", r.VMRef, "taskId", removeTaskID)
	taskInfo, err := r.getTaskByID(removeTaskID)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	return r.checkTaskStatus(taskInfo)
}

func (r *Client) checkTaskStatus(taskInfo *types.TaskInfo) (ready bool, err error) {
	switch taskInfo.State {
	case types.TaskInfoStateSuccess:
		return true, nil
	case types.TaskInfoStateError:
		return false, fmt.Errorf("error checking task status: %s", taskInfo.Error.LocalizedMessage)
	default:
		return false, nil
	}
}

func (r *Client) getTaskByID(taskID string) (*types.TaskInfo, error) {
	r.Log.V(1).Info("Get task by id", "taskId", taskID, "vmRef", r.VMRef)

	client := r.vim()
	pc := property.DefaultCollector(client)
	pc, err := pc.Create(context.TODO())
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer pc.Destroy(context.TODO())

	taskRef := types.ManagedObjectReference{
		Type:  taskType,
		Value: taskID,
	}
	var content []types.ObjectContent
	err = pc.RetrieveOne(context.TODO(), taskRef, []string{"info"}, &content)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if len(content[0].PropSet) == 0 {
		return nil, fmt.Errorf("task %s not found property set", taskID)
	}
	if content[0].PropSet[0].Val == nil {
		return nil, fmt.Errorf("no task value found for task %s", taskID)
	}
	task := content[0].PropSet[0].Val.(types.TaskInfo)
	return &task, nil
}

// Close logs out of the vSphere session.
func (r *Client) Close() {
	if r.client != nil {
		_ = r.client.Logout(context.TODO())
		r.client.CloseIdleConnections()
	}
}
