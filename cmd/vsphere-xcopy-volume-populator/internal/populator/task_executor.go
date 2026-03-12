package populator

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

// Unified progress pattern that handles both VIB and SSH output formats
var progressPattern = regexp.MustCompile(`(\d+)\%`)

// TaskExecutor abstracts the transport-specific operations for task execution
type TaskExecutor interface {
	// StartClone initiates the clone operation and returns task information
	StartClone(ctx context.Context, host *object.HostSystem, datastore, sourcePath, targetLUN string) (*vmkfstoolsTask, error)

	// GetTaskStatus retrieves the current status of the specified task
	GetTaskStatus(ctx context.Context, host *object.HostSystem, datastore, taskId string) (*vmkfstoolsTask, error)

	// CleanupTask cleans up task artifacts
	CleanupTask(ctx context.Context, host *object.HostSystem, datastore, taskId string) error
}

// ParseProgress extracts progress percentage from vmkfstools output
// Returns -1 if no progress is found, otherwise returns 0-100
func ParseProgress(lastLine string) (int, error) {
	if lastLine == "" {
		return -1, fmt.Errorf("lastLine is empty")
	}

	// VIB format: "Clone: 15% done."
	match := progressPattern.FindStringSubmatch(lastLine)
	if len(match) > 1 {
		progress, err := strconv.Atoi(match[1])
		if err == nil {
			klog.V(2).Infof("ParseProgress: extracted progress: %d%%", progress)
			return progress, nil
		}
		klog.Warningf("ParseProgress: failed to parse progress from %q: %v", match[1], err)
		return -1, fmt.Errorf("failed to parse progress number from %q: %v", match[1], err)
	}

	return -1, nil
}

func updateTaskStatus(ctx context.Context, task *vmkfstoolsTask, executor TaskExecutor, host *object.HostSystem, datastore string, progress chan<- uint64, xcopyUsed chan<- int) (*vmkfstoolsTask, error) {
	taskStatus, err := executor.GetTaskStatus(ctx, host, datastore, task.TaskId)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	klog.FromContext(ctx).V(2).Info("task status", "task_status", taskStatus)

	// Report progress if found
	if progressValue, err := ParseProgress(taskStatus.LastLine); err == nil {
		progress <- uint64(progressValue)
	}

	// Report xcopyUsed as 0 or 1
	// nil means unknown/not determined, treat as 0 (not used)
	if taskStatus.XcopyUsed != nil && *taskStatus.XcopyUsed {
		xcopyUsed <- 1
	} else {
		xcopyUsed <- 0
	}

	return taskStatus, nil
}

// ExecuteCloneTask handles the unified task execution logic
func ExecuteCloneTask(ctx context.Context, executor TaskExecutor, host *object.HostSystem, datastore, sourcePath, targetLUN string, progress chan<- uint64, xcopyUsed chan<- int) error {
	log := klog.FromContext(ctx)

	log.Info("starting clone task", "source", sourcePath, "target", targetLUN)
	task, err := executor.StartClone(ctx, host, datastore, sourcePath, targetLUN)
	if err != nil {
		return fmt.Errorf("failed to start clone task: %w", err)
	}

	log.Info("clone task started", "task_id", task.TaskId)

	if task.TaskId != "" {
		defer func() {
			err := executor.CleanupTask(ctx, host, datastore, task.TaskId)
			if err != nil {
				log.Info("cleanup task artifacts failed", "err", err)
			}
		}()
	}

	for {
		taskStatus, err := updateTaskStatus(ctx, task, executor, host, datastore, progress, xcopyUsed)
		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		if taskStatus != nil && taskStatus.ExitCode != "" {
			time.Sleep(taskPollingInterval)
			taskStatus, err := updateTaskStatus(ctx, task, executor, host, datastore, progress, xcopyUsed)
			if err != nil {
				return fmt.Errorf("failed to update task status: %w", err)
			}
			if taskStatus.ExitCode == "0" {
				log.Info("clone task finished")
				return nil
			}
			return fmt.Errorf("clone task failed with exit code %s, stderr: %s", taskStatus.ExitCode, taskStatus.Stderr)
		}

		time.Sleep(taskPollingInterval)
	}
}
