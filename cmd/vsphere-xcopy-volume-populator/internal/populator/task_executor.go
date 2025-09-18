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
var cloneProgressPattern = regexp.MustCompile(`(\d+)`)

// TaskInfo represents information about a started clone task
type TaskInfo struct {
	TaskId string
	Pid    int
}

// TaskStatus represents the current status of a clone task
type TaskStatus struct {
	TaskId       string
	ExitCode     string
	Stderr       string
	LastLine     string
	XcloneWrites string
	XcopyUsed    bool
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
func ParseProgress(lastLine string, xcloneWrites string) (int, int, error) {
	if lastLine == "" {
		return -1, -1, fmt.Errorf("lastLine is empty")
	}


	// Defaults when not found
	progress := -1
	cloneProgress := -1

	// VIB format: "Clone: 15% done."
	match := progressPattern.FindStringSubmatch(lastLine)
	if len(match) > 1 {
		tempProgress, err := strconv.Atoi(match[1])
		if err == nil {
			progress = tempProgress
			klog.Infof("ParseProgress: extracted progress: %d%%", progress)
		} else {
			klog.Warningf("ParseProgress: failed to parse progress number from %q: %v", match[1], err)
			return -1, -1, fmt.Errorf("failed to parse progress number from %q: %v", match[1], err)
		}
	}

	xcloneWritesMatch := cloneProgressPattern.FindStringSubmatch(xcloneWrites)
	if len(xcloneWritesMatch) > 1 {
		tempCloneProgress, err := strconv.Atoi(xcloneWritesMatch[1])
		if err == nil {
			cloneProgress = tempCloneProgress
			klog.Infof("ParseProgress: extracted clone bytes progress: %d", cloneProgress)
		} else {
			klog.Warningf("ParseProgress: failed to parse clone bytes progress number from %q: %v", xcloneWritesMatch[1], err)
			return -1, -1, fmt.Errorf("failed to parse clone bytes progress number from %q: %v", xcloneWritesMatch[1], err)
		}
	}

	return progress, cloneProgress, nil
}

func UpdateTaskStatus(ctx context.Context, task *TaskInfo, executor TaskExecutor, host *object.HostSystem, taskId string, progress chan<- uint64, cloneProgressBytes chan<- uint64) (*TaskStatus, error) {
	taskStatus, err := executor.GetTaskStatus(ctx, host, task.TaskId)
		if err != nil {
			return nil, fmt.Errorf("failed to get task status: %w", err)
		}

		klog.V(2).Infof("Task status: %+v", taskStatus)

		// Report progress if found
		if progressValue, cloneProgress, err := ParseProgress(taskStatus.LastLine, taskStatus.XcloneWrites); err == nil {
			progress <- uint64(progressValue)
			cloneProgressBytes <- uint64(cloneProgress)
		}

	return taskStatus, nil
}

// ExecuteCloneTask handles the unified task execution logic
func ExecuteCloneTask(ctx context.Context, executor TaskExecutor, host *object.HostSystem, sourcePath, targetLUN string, progress chan<- uint64, cloneProgressBytes chan<- uint64) error {
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
		taskStatus, err := UpdateTaskStatus(ctx, task, executor, host, task.TaskId, progress, cloneProgressBytes)
		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		// Check for task completion
		if taskStatus != nil && taskStatus.ExitCode != "" {
			time.Sleep(taskPollingInterval)
			taskStatus, err := UpdateTaskStatus(ctx, task, executor, host, task.TaskId, progress, cloneProgressBytes)
			if err != nil {
				return fmt.Errorf("failed to update task status: %w", err)
			}
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
