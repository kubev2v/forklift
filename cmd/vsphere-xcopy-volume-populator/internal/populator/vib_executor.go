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

func (e *VIBTaskExecutor) StartClone(ctx context.Context, host *object.HostSystem, _, sourcePath, targetLUN string) (*vmkfstoolsTask, error) {
	log := klog.FromContext(ctx)
	cloningCtx := klog.NewContext(ctx, log.WithName("cloning"))
	r, err := e.VSphereClient.RunEsxCommand(cloningCtx, host, []string{"vmkfstools", "clone", "-s", sourcePath, "-t", targetLUN})
	if err != nil {
		log.V(2).Info("VIB clone failed", "response", r, "err", err)
		return nil, err
	}

	response := ""
	for _, l := range r {
		response += l.Value("message")
	}

	t := vmkfstoolsTask{}
	err = json.Unmarshal([]byte(response), &t)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (e *VIBTaskExecutor) GetTaskStatus(ctx context.Context, host *object.HostSystem, _, taskId string) (*vmkfstoolsTask, error) {
	log := klog.FromContext(ctx)
	cloningCtx := klog.NewContext(ctx, log.WithName("cloning"))
	r, err := e.VSphereClient.RunEsxCommand(cloningCtx, host, []string{"vmkfstools", "taskGet", "-i", taskId})
	if err != nil {
		return nil, err
	}

	response := ""
	for _, l := range r {
		response += l.Value("message")
	}

	t := vmkfstoolsTask{}
	err = json.Unmarshal([]byte(response), &t)
	if err != nil {
		log.V(2).Info("failed to unmarshal task status", "response", r, "err", err)
		return nil, err
	}

	return &t, nil
}

func (e *VIBTaskExecutor) CleanupTask(ctx context.Context, host *object.HostSystem, datastore, taskId string) error {
	log := klog.FromContext(ctx)
	cloningCtx := klog.NewContext(ctx, log.WithName("cloning"))
	log.Info("cleaning up VIB task", "task_id", taskId)
	r, errClean := e.VSphereClient.RunEsxCommand(cloningCtx, host, []string{"vmkfstools", "taskClean", "-i", taskId})
	if errClean != nil {
		log.V(2).Info("VIB task clean failed", "task_id", taskId, "response", r, "err", errClean)
		return errClean
	}
	return nil
}
