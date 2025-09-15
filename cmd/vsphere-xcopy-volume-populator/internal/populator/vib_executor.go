package populator

import (
	"context"
	"encoding/json"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

// VIBTaskExecutor implements TaskExecutor for the VIB method
type VIBTaskExecutor struct {
	VSphereClient vmware.Client
}

func NewVIBTaskExecutor(client vmware.Client) TaskExecutor {
	return &VIBTaskExecutor{
		VSphereClient: client,
	}
}

func (e *VIBTaskExecutor) StartClone(ctx context.Context, host *object.HostSystem, sourcePath, targetLUN string) (*TaskInfo, error) {
	r, err := e.VSphereClient.RunEsxCommand(ctx, host, []string{"vmkfstools", "clone", "-s", sourcePath, "-t", targetLUN})
	if err != nil {
		klog.Infof("error during copy, response from esxcli %+v", r)
		return nil, err
	}

	response := ""
	klog.Info("response from esxcli ", r)
	for _, l := range r {
		response += l.Value("message")
	}

	v := vmkfstoolsClone{}
	err = json.Unmarshal([]byte(response), &v)
	if err != nil {
		return nil, err
	}

	return &TaskInfo{
		TaskId: v.TaskId,
		Pid:    v.Pid,
	}, nil
}

func (e *VIBTaskExecutor) GetTaskStatus(ctx context.Context, host *object.HostSystem, taskId string) (*TaskStatus, error) {
	r, err := e.VSphereClient.RunEsxCommand(ctx, host, []string{"vmkfstools", "taskGet", "-i", taskId})
	if err != nil {
		return nil, err
	}

	response := ""
	klog.Info("response from esxcli ", r)
	for _, l := range r {
		response += l.Value("message")
	}

	v := vmkfstoolsTask{}
	err = json.Unmarshal([]byte(response), &v)
	if err != nil {
		klog.Errorf("failed to unmarshal response from esxcli %+v", r)
		return nil, err
	}

	klog.Infof("response from esxcli %+v", v)

	return &TaskStatus{
		TaskId:    v.TaskId,
		ExitCode:  v.ExitCode,
		Stderr:    v.Stderr,
		LastLine:  v.LastLine,
		XcopyUsed: v.XcopyUsed,
	}, nil
}

func (e *VIBTaskExecutor) CleanupTask(ctx context.Context, host *object.HostSystem, taskId string) error {
	r, errClean := e.VSphereClient.RunEsxCommand(ctx, host, []string{"vmkfstools", "taskClean", "-i", taskId})
	if errClean != nil {
		klog.Errorf("failed cleaning up task artifacts %v", r)
		return errClean
	}
	return nil
}
