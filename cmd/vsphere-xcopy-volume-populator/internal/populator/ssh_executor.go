package populator

import (
	"context"
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

// SSHTaskExecutor implements TaskExecutor for the SSH method
type SSHTaskExecutor struct {
	SSHClient vmware.SSHClient
}

func NewSSHTaskExecutor(sshClient vmware.SSHClient) TaskExecutor {
	return &SSHTaskExecutor{
		SSHClient: sshClient,
	}
}

func (e *SSHTaskExecutor) StartClone(ctx context.Context, host *object.HostSystem, sourcePath, targetLUN string) (*TaskInfo, error) {
	task, err := e.SSHClient.StartVmkfstoolsClone(sourcePath, targetLUN)
	if err != nil {
		return nil, fmt.Errorf("failed to start vmkfstools clone: %w", err)
	}

	return &TaskInfo{
		TaskId: task.TaskId,
		Pid:    task.Pid,
	}, nil
}

func (e *SSHTaskExecutor) GetTaskStatus(ctx context.Context, host *object.HostSystem, taskId string) (*TaskStatus, error) {
	taskStatus, err := e.SSHClient.GetTaskStatus(taskId)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	return &TaskStatus{
		TaskId:   taskStatus.TaskId,
		ExitCode: taskStatus.ExitCode,
		Stderr:   taskStatus.Stderr,
		LastLine: taskStatus.LastLine,
		XcopyUsed: taskStatus.XcopyUsed,
		XcloneWrites: taskStatus.XcloneWrites,
	}, nil
}

func (e *SSHTaskExecutor) CleanupTask(ctx context.Context, host *object.HostSystem, taskId string) error {
	err := e.SSHClient.CleanupTask(taskId)
	if err != nil {
		klog.Errorf("Failed cleaning up task artifacts: %v", err)
		return err
	}
	return nil
}
