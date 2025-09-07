package vmware

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

// SSHOperation represents the type of SSH operation
type SSHOperation string

const (
	SSHOperationClone   SSHOperation = "clone"
	SSHOperationStatus  SSHOperation = "status"
	SSHOperationCleanup SSHOperation = "cleanup"
)

type VmkfstoolsTask struct {
	TaskId   string `json:"taskId"`
	Pid      int    `json:"pid"`
	ExitCode string `json:"exitCode"`
	LastLine string `json:"lastLine"`
	Stderr   string `json:"stdErr"`
}

// XMLResponse represents the XML response structure
type XMLResponse struct {
	XMLName   xml.Name  `xml:"o"`
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

// SSHClient interface for SSH operations
type SSHClient interface {
	Connect(ctx context.Context, hostname, username string, privateKey []byte) error
	StartVmkfstoolsClone(sourceVMDK, targetLUN string) (*VmkfstoolsTask, error)
	GetTaskStatus(taskId string) (*VmkfstoolsTask, error)
	CleanupTask(taskId string) error
	Close() error
}

type ESXiSSHClient struct {
	hostname   string
	username   string
	sshClient  *ssh.Client
	privateKey []byte
}

func NewSSHClient() SSHClient {
	return &ESXiSSHClient{}
}

func (c *ESXiSSHClient) Connect(ctx context.Context, hostname, username string, privateKey []byte) error {
	c.hostname = hostname
	c.username = username
	c.privateKey = privateKey

	// Parse the private key
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Establish TCP connection honoring context cancellation/deadline
	addr := net.JoinHostPort(hostname, "22")
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Ensure the SSH handshake also respects the context deadline
	if deadline, ok := ctx.Deadline(); ok {
		_ = netConn.SetDeadline(deadline)
	}

	// Perform SSH handshake on the established net.Conn
	cc, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		_ = netConn.Close()
		return fmt.Errorf("failed to establish SSH client connection: %w", err)
	}
	c.sshClient = ssh.NewClient(cc, chans, reqs)
	klog.Infof("Connected to SSH server %s", hostname)
	return nil
}

// executeCommand executes a command using the SSH_ORIGINAL_COMMAND pattern
func (c *ESXiSSHClient) executeCommand(sshCommand string, args ...string) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("SSH client not connected")
	}

	// Create a new session for this command
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Build the complete command with arguments
	fullCommand := sshCommand
	if len(args) > 0 {
		fullCommand = fmt.Sprintf("%s %s", sshCommand, strings.Join(args, " "))
	}

	klog.V(2).Infof("Executing SSH command: %s", fullCommand)

	// Create a context with timeout for the command execution
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Channel to receive the command result
	type commandResult struct {
		output []byte
		err    error
	}
	resultChan := make(chan commandResult, 1)

	// Execute command in a goroutine
	go func() {
		// The SSH command will be passed as SSH_ORIGINAL_COMMAND to the restricted script
		output, err := session.CombinedOutput(fullCommand)
		resultChan <- commandResult{output: output, err: err}
	}()

	// Wait for either the command to complete or timeout
	var output []byte
	var cmdErr error
	select {
	case result := <-resultChan:
		output = result.output
		cmdErr = result.err
	case <-ctx.Done():
		// Command timed out, try to close the session
		session.Close()
		return "", fmt.Errorf("SSH command timed out after 60 seconds: %s", fullCommand)
	}

	if cmdErr != nil {
		klog.Warningf("SSH command failed: %s, output: %s, error: %v", fullCommand, string(output), cmdErr)
		return string(output), cmdErr
	}

	klog.V(2).Infof("SSH command succeeded: %s, output: %s", fullCommand, string(output))
	return string(output), nil
}

func (c *ESXiSSHClient) StartVmkfstoolsClone(sourceVMDK, targetLUN string) (*VmkfstoolsTask, error) {
	klog.Infof("Starting vmkfstools clone: source=%s, target=%s", sourceVMDK, targetLUN)

	output, err := c.executeCommand(string(SSHOperationClone), sourceVMDK, targetLUN)
	if err != nil {
		return nil, fmt.Errorf("failed to start clone: %w", err)
	}

	klog.Infof("Received output from script: %s", output)

	// Parse the XML response from the script
	task, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse clone response: %w", err)
	}

	klog.Infof("Started vmkfstools clone task %s with PID %d", task.TaskId, task.Pid)
	return task, nil
}

func (c *ESXiSSHClient) GetTaskStatus(taskId string) (*VmkfstoolsTask, error) {
	klog.V(2).Infof("Getting task status for %s", taskId)

	output, err := c.executeCommand(string(SSHOperationStatus), taskId)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	// Parse the XML response from the script
	task, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	klog.V(2).Infof("Task %s status: PID=%d, ExitCode=%s, LastLine=%s",
		taskId, task.Pid, task.ExitCode, task.LastLine)

	return task, nil
}

func (c *ESXiSSHClient) CleanupTask(taskId string) error {
	klog.Infof("Cleaning up task %s", taskId)

	output, err := c.executeCommand(string(SSHOperationCleanup), taskId)
	if err != nil {
		return fmt.Errorf("failed to cleanup task: %w", err)
	}

	// Parse response to ensure cleanup was successful
	_, err = parseTaskResponse(output)
	if err != nil {
		klog.Warningf("Cleanup response parsing failed (task may still be cleaned): %v", err)
	}

	klog.Infof("Cleaned up task %s", taskId)
	return nil
}

func (c *ESXiSSHClient) Close() error {
	if c.sshClient != nil {
		err := c.sshClient.Close()
		c.sshClient = nil
		klog.Infof("Closed SSH connection to %s", c.hostname)
		return err
	}
	return nil
}

// parseTaskResponse parses the XML response from the script
func parseTaskResponse(xmlOutput string) (*VmkfstoolsTask, error) {
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

	// Check if operation was successful
	if status != "success" {
		return nil, fmt.Errorf("operation failed with status %s: %s", status, message)
	}

	// Parse the JSON message to extract task information
	task := &VmkfstoolsTask{}

	// Try to parse as JSON first
	if err := json.Unmarshal([]byte(message), task); err != nil {
		// If JSON parsing fails, check if it's a simple text message (e.g., for cleanup operations)
		// In this case, we return a minimal task structure
		klog.V(2).Infof("Message is not JSON, treating as plain text: %s", message)

		// For non-JSON messages (like cleanup confirmations), return a basic task
		// The caller should check the original status for success/failure
		return &VmkfstoolsTask{
			LastLine: message, // Store the text message in LastLine for reference
		}, nil
	}

	return task, nil
}

// EnableSSHAccess enables SSH service on ESXi host and provides manual SSH key installation instructions
func EnableSSHAccess(ctx context.Context, vmwareClient Client, host *object.HostSystem, privateKey, publicKey []byte, scriptPath string) error {
	publicKeyStr := strings.TrimSpace(string(publicKey))

	klog.Infof("Enabling SSH access on ESXi host %s", host.Name())

	// Step 1: Get host IP address for SSH connectivity testing
	hostIP, err := GetHostIPAddress(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get host IP address: %w", err)
	}

	// Step 2: Check ESXi version
	version, err := getESXiVersion(vmwareClient, host, ctx)
	if err != nil {
		return fmt.Errorf("failed to get ESXi version: %w", err)
	}
	klog.Infof("ESXi version %s detected", version)

	pythonCommand := fmt.Sprintf("python %s", scriptPath)
	restrictedPublicKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`,
		pythonCommand, publicKeyStr)

	// Step 7: Test SSH connectivity first (using private key for authentication)
	if testSSHConnectivity(ctx, hostIP, privateKey, "test") {
		klog.Infof("SSH connectivity test passed - keys already configured correctly")
		return nil
	}

	// Step 8: Manual SSH key installation required for all ESXi versions
	klog.Errorf("Manual SSH key installation required. Please add the following line to /etc/ssh/keys-root/authorized_keys on the ESXi host:")
	klog.Errorf("")
	klog.Errorf("  %s", restrictedPublicKey)
	klog.Errorf("")
	klog.Errorf("Steps to manually configure SSH key:")
	klog.Errorf("1. SSH to the ESXi host: ssh root@%s", hostIP)
	klog.Errorf("2. Edit the authorized_keys file: vi /etc/ssh/keys-root/authorized_keys")
	klog.Errorf("3. Add the above line to the file")
	klog.Errorf("4. Save and exit")
	klog.Errorf("5. Restart the operation")
	return fmt.Errorf("manual SSH key configuration required for ESXi %s - see logs for instructions", version)
}

// testSSHConnectivity tests if we can connect via SSH and execute a restricted command
func testSSHConnectivity(ctx context.Context, hostIP string, privateKey []byte, testCommand string) bool {
	klog.Infof("Testing SSH connectivity to %s", hostIP)

	client := NewSSHClient()
	// Use a short timeout for connectivity testing to avoid indefinite hangs
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := client.Connect(testCtx, hostIP, "root", privateKey)
	if err != nil {
		klog.Infof("SSH connectivity test failed to connect: %v", err)
		return false
	}
	defer client.Close()

	// Try to execute a simple test command - this will test if the restricted command setup is working
	// We expect this to fail with a "task not found" type error, which indicates SSH restrictions are working correctly
	output, err := client.(*ESXiSSHClient).executeCommand(string(SSHOperationStatus), "test-task-id")

	klog.Infof("SSH test command output: '%s'", output)
	klog.Infof("SSH test command error: %v", err)

	if strings.Contains(output, "<?xml version=") {
		klog.Infof("Received XML response from script - SSH working correctly")
		return true
	}

	if strings.Contains(output, "No such file or directory") && strings.Contains(output, ".py") {
		klog.Infof("SSH working but script file not found - configuration issue")
		return true
	}

	klog.Infof("SSH connectivity issue detected, none of the expecter responses were received: %v, err: %v", output, err)
	return false
}

// getESXiVersion retrieves the ESXi version from the host
func getESXiVersion(vmwareClient Client, host *object.HostSystem, ctx context.Context) (string, error) {
	command := []string{"system", "version", "get"}
	output, err := vmwareClient.RunEsxCommand(ctx, host, command)
	if err != nil {
		return "", fmt.Errorf("failed to get ESXi version: %w", err)
	}

	for _, valueMap := range output {
		if version, exists := valueMap["Version"]; exists && len(version) > 0 {
			return version[0], nil
		}
		if product, exists := valueMap["Product"]; exists && len(product) > 0 {
			if strings.Contains(product[0], "ESXi") {
				if versionField, versionExists := valueMap["Version"]; versionExists && len(versionField) > 0 {
					return versionField[0], nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not parse ESXi version from command output")
}

// GetHostIPAddress retrieves the management IP address of an ESXi host
func GetHostIPAddress(ctx context.Context, host *object.HostSystem) (string, error) {
	ips, err := host.ManagementIPs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get management IPs: %w", err)
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no management IP addresses found")
	}

	return ips[0].String(), nil
}
