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
	taskPaths map[string]string
}

func NewSSHTaskExecutor(sshClient vmware.SSHClient) TaskExecutor {
	return &SSHTaskExecutor{
		sshClient: sshClient,
		taskPaths: make(map[string]string),
	}
}

func (e *SSHTaskExecutor) StartClone(_ context.Context, _ *object.HostSystem, sourcePath, targetLUN string) (*vmkfstoolsTask, error) {
	klog.Infof("Starting vmkfstools clone: source=%s, target=%s", sourcePath, targetLUN)
	output, err := e.sshClient.ExecuteCommand("--clone", "-s", sourcePath, "-t", targetLUN)
	if err != nil {
		return nil, fmt.Errorf("failed to start clone: %w", err)
	}

	klog.Infof("Received output from script: %s", output)

	t, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse clone response: %w", err)
	}
	c, err := parseTaskPathResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task path response: %w", err)
	}
	e.taskPaths[t.TaskId] = c.TaskPath
	klog.Infof("Started vmkfstools clone task %s with PID %d", t.TaskId, t.Pid)
	return t, nil
}

func (e *SSHTaskExecutor) GetTaskStatus(_ context.Context, _ *object.HostSystem, taskId string) (*vmkfstoolsTask, error) {
	klog.V(2).Infof("Getting task status for %s", taskId)

	taskPath, ok := e.taskPaths[taskId]
	if !ok {
		return nil, fmt.Errorf("task path not found for task id %s", taskId)
	}
	output, err := e.sshClient.ExecuteCommand("--task-get", "-p", taskPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	t, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	klog.V(2).Infof("Task %s status: PID=%d, ExitCode=%s, LastLine=%s",
		taskId, t.Pid, t.ExitCode, t.LastLine)

	return t, nil
}

func (e *SSHTaskExecutor) CleanupTask(ctx context.Context, host *object.HostSystem, taskId string) error {
	klog.Infof("Cleaning up task %s", taskId)

	taskPath, ok := e.taskPaths[taskId]
	if !ok {
		return fmt.Errorf("task path not found for task id %s", taskId)
	}
	output, err := e.sshClient.ExecuteCommand("--task-clean", "-p", taskPath)
	if err != nil {
		return fmt.Errorf("failed to cleanup task: %w", err)
	}

	_, err = parseTaskResponse(output)
	if err != nil {
		klog.Warningf("Cleanup response parsing failed (task may still be cleaned): %v", err)
	}

	klog.Infof("Cleaned up task %s", taskId)
	return nil
}

// XMLResponse represents the XML response structure
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

// parseTaskResponse parses the XML response from the script
func parseXmlResponse(xmlOutput string) (string, string, error) {
	// Parse the XML response to extract the JSON result
	// Expected format: XML with status and message fields
	// The message field contains JSON with task information

	var response XMLResponse
	if err := xml.Unmarshal([]byte(xmlOutput), &response); err != nil {
		return "", "", fmt.Errorf("failed to parse XML response: %w", err)
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
		return "", "", fmt.Errorf("status field not found in XML response")
	}

	if message == "" {
		return "", "", fmt.Errorf("message field not found in XML response")
	}

	// Check if operation was successful (script returns "0" for success)
	if status != "0" {
		return "", "", fmt.Errorf("operation failed with status %s: %s", status, message)
	}
	return status, message, nil
}

func parseTaskResponse(xmlOutput string) (*vmkfstoolsTask, error) {
	// Parse the JSON message to extract task information
	task := &vmkfstoolsTask{}
	_, message, err := parseXmlResponse(xmlOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}
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

func parseTaskPathResponse(xmlOutput string) (*vmkfstoolsTaskPath, error) {
	taskPath := &vmkfstoolsTaskPath{}
	status, message, err := parseXmlResponse(xmlOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}
	if status != "0" {
		return nil, fmt.Errorf("operation failed with status %s: %s", status, message)
	}
	// Try to parse as JSON fist
	if err := json.Unmarshal([]byte(message), taskPath); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return taskPath, nil
}
