package populator

import (
	"context"
	"fmt"

	"encoding/json"
	"encoding/xml"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/object"

	"k8s.io/klog/v2"
)

// SSHTaskExecutor implements TaskExecutor for the SSH method
type SSHTaskExecutor struct {
	sshClient vmware.SSHClient
}

// XMLResponse represents the XML response structure from vmkfstools-wrapper script
type XMLResponse struct {
	XMLName   xml.Name  `xml:"output"`
	Structure Structure `xml:"structure"`
}

// Structure represents the structure element in the XML response
type Structure struct {
	TypeName string  `xml:"typeName,attr"`
	Fields   []Field `xml:"field"`
}

// Field represents a field in the XML response
type Field struct {
	Name   string `xml:"name,attr"`
	String string `xml:"string"`
}

func NewSSHTaskExecutor(sshClient vmware.SSHClient) TaskExecutor {
	return &SSHTaskExecutor{
		sshClient: sshClient,
	}
}

func (e *SSHTaskExecutor) StartClone(ctx context.Context, _ *object.HostSystem, datastore, sourcePath, targetLUN string) (*vmkfstoolsTask, error) {
	log := klog.FromContext(ctx)
	cloningCtx := klog.NewContext(ctx, log.WithName("cloning"))
	log.Info("starting clone via SSH", "datastore", datastore, "source", sourcePath, "target", targetLUN)

	output, err := e.sshClient.ExecuteCommand(cloningCtx, datastore, "--clone", "-s", sourcePath, "-t", targetLUN)
	if err != nil {
		return nil, fmt.Errorf("failed to start clone: %w", err)
	}

	t, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse clone response: %w", err)
	}

	log.V(2).Info("clone task started via SSH", "task_id", t.TaskId, "pid", t.Pid)
	return t, nil
}

func (e *SSHTaskExecutor) GetTaskStatus(ctx context.Context, _ *object.HostSystem, datastore, taskId string) (*vmkfstoolsTask, error) {
	log := klog.FromContext(ctx)
	cloningCtx := klog.NewContext(ctx, log.WithName("cloning"))
	log.V(2).Info("getting task status", "task_id", taskId, "datastore", datastore)

	output, err := e.sshClient.ExecuteCommand(cloningCtx, datastore, "--task-get", "-i", taskId)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	t, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	log.V(2).Info("task status", "task_id", taskId, "pid", t.Pid, "exit_code", t.ExitCode, "last_line", t.LastLine)

	return t, nil
}

func (e *SSHTaskExecutor) CleanupTask(ctx context.Context, _ *object.HostSystem, datastore, taskId string) error {
	log := klog.FromContext(ctx)
	cloningCtx := klog.NewContext(ctx, log.WithName("cloning"))
	log.Info("cleaning up task", "task_id", taskId, "datastore", datastore)

	output, err := e.sshClient.ExecuteCommand(cloningCtx, datastore, "--task-clean", "-i", taskId)
	if err != nil {
		return fmt.Errorf("failed to cleanup task: %w", err)
	}

	_, err = parseTaskResponse(output)
	if err != nil {
		log.V(2).Info("cleanup response parse failed (task may still be cleaned)", "err", err)
	}

	log.Info("cleaned up task", "task_id", taskId)
	return nil
}

// parseTaskResponse parses the XML response from the script
func parseTaskResponse(xmlOutput string) (*vmkfstoolsTask, error) {
	// Parse the XML response to extract the JSON result
	// Expected format: XML with status and message fields
	// The message field contains JSON with task information

	var response XMLResponse
	if err := xml.Unmarshal([]byte(xmlOutput), &response); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Find status and message fields
	var status, message string
	for _, field := range response.Structure.Fields {
		switch field.Name {
		case "status":
			status = field.String
		case "message":
			message = field.String
		}
	}

	if status == "" {
		return nil, fmt.Errorf("status field not found in XML response")
	}

	if message == "" {
		return nil, fmt.Errorf("message field not found in XML response")
	}

	// Check if operation was successful (script returns "0" for success)
	if status != "0" {
		return nil, fmt.Errorf("operation failed with status %s: %s", status, message)
	}

	// Parse the JSON message to extract task information
	task := &vmkfstoolsTask{}

	// Try to parse as JSON first
	if err := json.Unmarshal([]byte(message), task); err != nil {
		// If JSON parsing fails, check if it's a simple text message (e.g., for cleanup operations)
		// In this case, we return a minimal task structure
		klog.V(2).Infof("Message is not JSON, treating as plain text: %s", message)

		// For non-JSON messages (like cleanup confirmations), return a basic task
		// The caller should check the original status for success/failure
		return &vmkfstoolsTask{
			LastLine: message, // Store the text message in LastLine for reference
		}, nil
	}

	return task, nil
}
