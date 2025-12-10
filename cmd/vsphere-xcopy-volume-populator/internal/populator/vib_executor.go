package populator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

// VIBTaskExecutor implements TaskExecutor for the VIB method
type VIBTaskExecutor struct {
	VSphereClient vmware.Client
	taskPaths     map[string]string
}

func NewVIBTaskExecutor(client vmware.Client) TaskExecutor {
	return &VIBTaskExecutor{
		VSphereClient: client,
		taskPaths:     make(map[string]string),
	}
}

func (e *VIBTaskExecutor) StartClone(ctx context.Context, host *object.HostSystem, sourcePath, targetLUN string) (*vmkfstoolsTask, error) {
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

	t := vmkfstoolsTask{}
	err = json.Unmarshal([]byte(response), &t)
	if err != nil {
		return nil, err
	}
	c := vmkfstoolsTaskPath{}
	err = json.Unmarshal([]byte(response), &c)
	if err != nil {
		return nil, err
	}
	e.taskPaths[t.TaskId] = c.TaskPath
	return &t, nil
}

func (e *VIBTaskExecutor) GetTaskStatus(ctx context.Context, host *object.HostSystem, taskId string) (*vmkfstoolsTask, error) {
	taskPath, ok := e.taskPaths[taskId]
	if !ok {
		return nil, fmt.Errorf("task path not found for task id %s", taskId)
	}
	r, err := e.VSphereClient.RunEsxCommand(ctx, host, []string{"vmkfstools", "taskGet", "-p", taskPath})
	if err != nil {
		return nil, err
	}

	response := ""
	klog.Info("response from esxcli ", r)
	for _, l := range r {
		response += l.Value("message")
	}

	t := vmkfstoolsTask{}
	err = json.Unmarshal([]byte(response), &t)
	if err != nil {
		klog.Errorf("failed to unmarshal response from esxcli %+v", r)
		return nil, err
	}

	klog.Infof("response from esxcli %+v", t)

	return &t, nil
}

func (e *VIBTaskExecutor) CleanupTask(ctx context.Context, host *object.HostSystem, taskId string) error {
	taskPath, ok := e.taskPaths[taskId]
	if !ok {
		return fmt.Errorf("task path not found for task id %s", taskId)
	}
	r, errClean := e.VSphereClient.RunEsxCommand(ctx, host, []string{"vmkfstools", "taskClean", "-p", taskPath})
	if errClean != nil {
		klog.Errorf("failed cleaning up task artifacts %v", r)
		return errClean
	}
	return nil
}
