package populator

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
)

const (
	// scratchFolder is the datastore folder the transient RDM mapping files are
	// created in. Keeping them out of the source VM's folder means a descriptor
	// leaked by a crashed run is identifiable and sweepable rather than sitting
	// next to production VM files.
	scratchFolder = "forklift-xcopy"

	// rdmAdapterType only describes the virtual adapter recorded in the mapping
	// file. The copy itself does not go through it.
	rdmAdapterType = "lsiLogic"

	// rdmDiskType is virtual compatibility mode. The mapping exists only for the
	// duration of the copy and is never attached to a VM, so the snapshot and
	// SCSI pass-through differences against physical mode ("rdmp") do not apply.
	rdmDiskType = string(types.VirtualDiskTypeRdm)
)

// APITaskExecutor implements TaskExecutor on top of
// VirtualDiskManager.CopyVirtualDisk, with no VIB and no SSH.
//
// The API cannot name a raw device as a copy destination -- dst is always a
// datastore path to a virtual disk. The only way to address the target LUN is
// therefore to make the destination a raw disk mapping: a small descriptor on
// VMFS whose Device field points at /vmfs/devices/disks/naa.X. The data lands on
// the LUN; the descriptor is just the handle, and CleanupTask removes it.
type APITaskExecutor struct {
	hostClient *vmware.HostClient
	// descriptorName is derived from the populator CR, so it is stable across
	// retries of the same disk and unique between concurrent disks.
	descriptorName string

	// descriptorPath is recorded by StartClone so CleanupTask can find it.
	descriptorPath string
}

func NewAPITaskExecutor(hostClient *vmware.HostClient, descriptorName string) TaskExecutor {
	return &APITaskExecutor{
		hostClient:     hostClient,
		descriptorName: descriptorName,
	}
}

// toDatastorePath converts the /vmfs/volumes/<ds>/<rest> form that vmkfstools
// takes into the "[<ds>] <rest>" form the vSphere API requires.
func toDatastorePath(datastore, path string) (string, error) {
	prefix := fmt.Sprintf("/vmfs/volumes/%s/", datastore)
	rest, found := strings.CutPrefix(path, prefix)
	if !found || rest == "" {
		return "", fmt.Errorf("path %q is not under datastore %q", path, datastore)
	}
	return fmt.Sprintf("[%s] %s", datastore, rest), nil
}

func (e *APITaskExecutor) StartClone(ctx context.Context, _ *object.HostSystem, datastore, sourcePath, targetLUN string) (*vmkfstoolsTask, error) {
	log := klog.FromContext(ctx)

	source, err := toDatastorePath(datastore, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source datastore path: %w", err)
	}

	folder := fmt.Sprintf("[%s] %s", datastore, scratchFolder)
	e.descriptorPath = fmt.Sprintf("%s/%s.vmdk", folder, e.descriptorName)

	fm := object.NewFileManager(e.hostClient.Client.Client)
	dc := e.hostClient.Datacenter

	// MakeDirectory fails with FileAlreadyExists once the folder is there, which
	// is the steady state after the first disk on this datastore.
	if err := fm.MakeDirectory(ctx, folder, dc, true); err != nil {
		if !isAlreadyExists(err) {
			return nil, fmt.Errorf("failed to create scratch folder %q: %w", folder, err)
		}
	}

	// A descriptor at this path can only be debris from an earlier attempt at
	// this same disk: the name is derived from the populator CR. Clear it so the
	// copy does not fail on an existing destination. If it cannot be removed the
	// CopyVirtualDisk call below reports the real problem, so only log here.
	if err := e.removeDescriptor(ctx, "stale descriptor from a previous attempt"); err != nil {
		log.V(2).Info("could not clear a pre-existing descriptor", "descriptor", e.descriptorPath, "err", err)
	}

	// Ask the host what it calls this device rather than assuming the
	// /vmfs/devices/disks/<naa> spelling, and make sure hostd has actually
	// noticed the LUN we just mapped -- it validates spec.device against its own
	// cached inventory, not against what esxcli can see.
	canonicalName := path.Base(targetLUN)
	device, err := e.hostClient.ResolveDeviceName(ctx, canonicalName)
	if err != nil {
		return nil, fmt.Errorf("cannot use %s as the clone destination: %w", canonicalName, err)
	}
	if device != targetLUN {
		log.Info("host reports a different device name than the constructed path",
			"constructed", targetLUN, "hostReported", device)
	}

	destSpec := &types.DeviceBackedVirtualDiskSpec{
		VirtualDiskSpec: types.VirtualDiskSpec{
			DiskType:    rdmDiskType,
			AdapterType: rdmAdapterType,
		},
		Device: device,
	}

	log.Info("starting clone via vSphere API",
		"source", source, "descriptor", e.descriptorPath, "device", device, "rdmDiskType", rdmDiskType)

	vdm := object.NewVirtualDiskManager(e.hostClient.Client.Client)
	task, err := vdm.CopyVirtualDisk(ctx, source, dc, e.descriptorPath, dc, destSpec, true)
	if err != nil {
		return nil, fmt.Errorf("CopyVirtualDisk failed for %s -> %s: %w", source, targetLUN, err)
	}

	taskId := task.Reference().Value
	log.V(2).Info("clone task started via API", "task_id", taskId)
	return &vmkfstoolsTask{TaskId: taskId}, nil
}

func (e *APITaskExecutor) GetTaskStatus(ctx context.Context, _ *object.HostSystem, _, taskId string) (*vmkfstoolsTask, error) {
	var t mo.Task
	ref := types.ManagedObjectReference{Type: "Task", Value: taskId}
	pc := property.DefaultCollector(e.hostClient.Client.Client)
	if err := pc.RetrieveOne(ctx, ref, []string{"info"}, &t); err != nil {
		return nil, fmt.Errorf("failed to retrieve task %s: %w", taskId, err)
	}

	status := &vmkfstoolsTask{TaskId: taskId}

	// ParseProgress reads a "NN%" substring, so rendering progress into LastLine
	// keeps the shared polling loop in ExecuteCloneTask working unchanged.
	if t.Info.Progress >= 0 {
		status.LastLine = fmt.Sprintf("%d%%", t.Info.Progress)
	}

	switch t.Info.State {
	case types.TaskInfoStateSuccess:
		status.ExitCode = "0"
		status.LastLine = "100%"
	case types.TaskInfoStateError:
		status.ExitCode = "1"
		if t.Info.Error != nil {
			status.Stderr = t.Info.Error.LocalizedMessage
		}
		if status.Stderr == "" {
			status.Stderr = "CopyVirtualDisk task failed without a reported fault"
		}
	default:
		// queued or running: empty ExitCode keeps ExecuteCloneTask polling.
	}

	return status, nil
}

// CleanupTask removes the RDM mapping file. The target LUN keeps the copied
// data: deleting a raw disk mapping drops the pointer, not the contents of the
// device it points at.
func (e *APITaskExecutor) CleanupTask(ctx context.Context, _ *object.HostSystem, _, _ string) error {
	return e.removeDescriptor(ctx, "clone finished")
}

// removeDescriptor deletes the mapping file if it exists. A missing file is the
// normal case on the pre-clone sweep, so it is not treated as an error.
func (e *APITaskExecutor) removeDescriptor(ctx context.Context, reason string) error {
	if e.descriptorPath == "" {
		return nil
	}
	log := klog.FromContext(ctx)

	vdm := object.NewVirtualDiskManager(e.hostClient.Client.Client)
	task, err := vdm.DeleteVirtualDisk(ctx, e.descriptorPath, e.hostClient.Datacenter)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete RDM descriptor %q: %w", e.descriptorPath, err)
	}
	if err := task.Wait(ctx); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete RDM descriptor %q: %w", e.descriptorPath, err)
	}

	log.Info("removed RDM descriptor", "descriptor", e.descriptorPath, "reason", reason)
	return nil
}

func isAlreadyExists(err error) bool {
	if soap.IsSoapFault(err) {
		switch soap.ToSoapFault(err).VimFault().(type) {
		case types.FileAlreadyExists:
			return true
		}
	}
	return strings.Contains(err.Error(), "already exists")
}

func isNotFound(err error) bool {
	if soap.IsSoapFault(err) {
		switch soap.ToSoapFault(err).VimFault().(type) {
		case types.FileNotFound, types.NotFound:
			return true
		}
	}
	return strings.Contains(err.Error(), "was not found") || strings.Contains(err.Error(), "does not exist")
}
