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
var unifiedProgressPattern = regexp.MustCompile(`(\d+)%`)

// TaskInfo represents information about a started clone task
type TaskInfo struct {
	TaskId string
	Pid    int
}

// TaskStatus represents the current status of a clone task
type TaskStatus struct {
	TaskId   string
	ExitCode string
	Stderr   string
	LastLine string
}

// TaskExecutor abstracts the transport-specific operations for task execution
type TaskExecutor interface {
	// StartClone initiates the clone operation and returns task information
	StartClone(ctx context.Context, host *object.HostSystem, sourcePath, targetLUN string) (*TaskInfo, error)

	// GetTaskStatus retrieves the current status of the specified task
	GetTaskStatus(ctx context.Context, host *object.HostSystem, taskId string) (*TaskStatus, error)

	// CleanupTask cleans up task artifacts
	CleanupTask(ctx context.Context, host *object.HostSystem, taskId string) error
}

// ParseProgress extracts progress percentage from vmkfstools output
// Returns -1 if no progress is found, otherwise returns 0-100
func ParseProgress(lastLine string) int {
	if lastLine == "" {
		return -1
	}

	klog.V(2).Infof("ParseProgress: parsing line: %q", lastLine)

	// VIB format: "Clone: 15% done."
	match := unifiedProgressPattern.FindStringSubmatch(lastLine)
	if len(match) > 1 {
		if progress, err := strconv.Atoi(match[1]); err == nil {
			klog.V(2).Infof("ParseProgress: extracted progress: %d%%", progress)
			return progress
		} else {
			klog.Warningf("ParseProgress: failed to parse progress number from %q: %v", match[1], err)
		}
	}

	klog.V(2).Infof("ParseProgress: no progress pattern found in line")
	return -1
}

// ExecuteCloneTask handles the unified task execution logic
func ExecuteCloneTask(ctx context.Context, executor TaskExecutor, host *object.HostSystem, sourcePath, targetLUN string, progress chan<- uint) error {
	// Start the clone task
	task, err := executor.StartClone(ctx, host, sourcePath, targetLUN)
	if err != nil {
		return fmt.Errorf("failed to start clone task: %w", err)
	}

	klog.Infof("Started clone task %s", task.TaskId)

	// Cleanup task artifacts when done
	if task.TaskId != "" {
		defer func() {
			err := executor.CleanupTask(ctx, host, task.TaskId)
			if err != nil {
				klog.Errorf("Failed cleaning up task artifacts: %v", err)
			}
		}()
	}

	// Poll for task completion
	for {
		taskStatus, err := executor.GetTaskStatus(ctx, host, task.TaskId)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		klog.V(2).Infof("Task status: %+v", taskStatus)

		// Report progress if found
		if progressValue := ParseProgress(taskStatus.LastLine); progressValue >= 0 {
			progress <- uint(progressValue)
		}

		// Check for task completion
		if taskStatus.ExitCode != "" {
			if taskStatus.ExitCode == "0" {
				klog.Infof("Clone task completed successfully")
				return nil
			} else {
				return fmt.Errorf("clone task failed with exit code %s, stderr: %s", taskStatus.ExitCode, taskStatus.Stderr)
			}
		}

		time.Sleep(taskPollingInterval)
	}
}
