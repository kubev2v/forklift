package vmware

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

var progressPattern = regexp.MustCompile(`Clone:\s(\d+)%\sdone\.`)

type VmkfstoolsTask struct {
	TaskId   string `json:"taskId"`
	Pid      int    `json:"pid"`
	ExitCode string `json:"exitCode"`
	LastLine string `json:"lastLine"`
	Stderr   string `json:"stdErr"`
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
func (c *ESXiSSHClient) executeCommand(operation, arg1, arg2 string) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("SSH client not connected")
	}

	// Create a new session for this command
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Build the command based on operation type
	var sshCommand string
	switch operation {
	case "clone":
		if arg2 == "" {
			return "", fmt.Errorf("clone operation requires both source and target arguments")
		}
		sshCommand = fmt.Sprintf("clone %s %s", arg1, arg2)
	case "status", "cleanup":
		if arg1 == "" {
			return "", fmt.Errorf("%s operation requires task ID argument", operation)
		}
		sshCommand = fmt.Sprintf("%s %s", operation, arg1)
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}

	klog.V(2).Infof("Executing SSH command: %s", sshCommand)

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
		output, err := session.CombinedOutput(sshCommand)
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
		return "", fmt.Errorf("SSH command timed out after 60 seconds: %s", sshCommand)
	}

	if cmdErr != nil {
		klog.Warningf("SSH command failed: %s, output: %s, error: %v", sshCommand, string(output), cmdErr)
		return string(output), cmdErr
	}

	klog.V(2).Infof("SSH command succeeded: %s, output: %s", sshCommand, string(output))
	return string(output), nil
}

func (c *ESXiSSHClient) StartVmkfstoolsClone(sourceVMDK, targetLUN string) (*VmkfstoolsTask, error) {
	klog.Infof("Starting vmkfstools clone: source=%s, target=%s", sourceVMDK, targetLUN)

	output, err := c.executeCommand("clone", sourceVMDK, targetLUN)
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

	output, err := c.executeCommand("status", taskId, "")
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

	output, err := c.executeCommand("cleanup", taskId, "")
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

	// Simple XML parsing for our specific format
	lines := strings.Split(xmlOutput, "\n")
	var statusLine, messageLine string

	for _, line := range lines {
		if strings.Contains(line, "<field name=\"status\">") {
			statusLine = line
		}
		if strings.Contains(line, "<field name=\"message\">") {
			messageLine = line
		}
	}

	// Extract status
	statusPattern := regexp.MustCompile(`<string>([^<]+)</string>`)
	statusMatches := statusPattern.FindStringSubmatch(statusLine)
	if len(statusMatches) < 2 {
		return nil, fmt.Errorf("failed to parse status from response")
	}
	status := statusMatches[1]

	// Extract message (JSON)
	messageMatches := statusPattern.FindStringSubmatch(messageLine)
	if len(messageMatches) < 2 {
		return nil, fmt.Errorf("failed to parse message from response")
	}
	message := messageMatches[1]

	// Check if operation was successful
	if status != "success" {
		return nil, fmt.Errorf("operation failed with status %s: %s", status, message)
	}

	// Parse the JSON message to extract task information
	// Simple JSON parsing for our specific format
	task := &VmkfstoolsTask{}

	// Extract taskId
	if taskIdMatch := regexp.MustCompile(`"taskId":\s*"([^"]+)"`).FindStringSubmatch(message); len(taskIdMatch) > 1 {
		task.TaskId = taskIdMatch[1]
	}

	// Extract pid
	if pidMatch := regexp.MustCompile(`"pid":\s*([0-9]+)`).FindStringSubmatch(message); len(pidMatch) > 1 {
		if pid, err := strconv.Atoi(pidMatch[1]); err == nil {
			task.Pid = pid
		}
	}

	// Extract exitCode
	if exitCodeMatch := regexp.MustCompile(`"exitCode":\s*"([^"]*)"`).FindStringSubmatch(message); len(exitCodeMatch) > 1 {
		task.ExitCode = exitCodeMatch[1]
	}

	// Extract lastLine
	if lastLineMatch := regexp.MustCompile(`"lastLine":\s*"([^"]*)"`).FindStringSubmatch(message); len(lastLineMatch) > 1 {
		task.LastLine = lastLineMatch[1]
	}

	// Extract stderr
	if stderrMatch := regexp.MustCompile(`"stdErr":\s*"([^"]*)"`).FindStringSubmatch(message); len(stderrMatch) > 1 {
		task.Stderr = stderrMatch[1]
	}

	return task, nil
}

// EnableSSHAccess enables SSH service on ESXi host and handles SSH key installation based on ESXi version
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

	// Step 3: Enable SSH service using proper vSphere API
	err = enableSSHService(vmwareClient, host, ctx)
	if err != nil {
		return fmt.Errorf("failed to enable SSH service: %w", err)
	}

	// Step 4: Configure firewall and system settings using vSphere API
	err = configureSSHFirewall(host, ctx)
	if err != nil {
		klog.Warningf("Failed to configure SSH firewall: %v", err)
	}

	// Step 6: Create SSH command with Python interpreter and restricted access
	pythonCommand := fmt.Sprintf("python %s", scriptPath)
	restrictedPublicKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`,
		pythonCommand, publicKeyStr)

	// Step 7: Test SSH connectivity first (using private key for authentication)
	if testSSHConnectivity(ctx, hostIP, privateKey, "test") {
		klog.Infof("SSH connectivity test passed - keys already configured correctly")
		return nil
	}

	// Step 8: Handle SSH key installation based on ESXi version
	if isESXi8OrNewer(version) {
		klog.Infof("ESXi %s detected - attempting automatic SSH key installation", version)
		err = installSSHKey(vmwareClient, host, restrictedPublicKey)
		if err != nil {
			return fmt.Errorf("failed to install restricted SSH key: %w", err)
		}
		klog.Infof("SSH key installed automatically for ESXi %s", version)
	} else {
		klog.Errorf("ESXi %s detected - automatic SSH key installation not supported", version)
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

	// Step 9: Test connectivity after installation (ESXi 8 only) - using private key
	if !testSSHConnectivity(ctx, hostIP, privateKey, "test") {
		return fmt.Errorf("SSH connectivity test failed after key installation")
	}

	klog.Infof("SSH access configured successfully on ESXi host %s", host.Name())
	return nil
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
	output, err := client.(*ESXiSSHClient).executeCommand("status", "test-task-id", "")

	klog.Infof("SSH test command output: '%s'", output)
	klog.Infof("SSH test command error: %v", err)

	if err != nil {
		// Analyze the error to determine if it's a connectivity issue or expected script behavior
		errorStr := strings.ToLower(err.Error())

		// These errors indicate SSH key/connectivity problems
		if strings.Contains(errorStr, "permission denied") ||
			strings.Contains(errorStr, "connection refused") ||
			strings.Contains(errorStr, "host key verification failed") ||
			strings.Contains(errorStr, "authentication failed") {
			klog.Infof("SSH connectivity issue detected: %v", err)
			return false
		}

		// These errors indicate the script is running but failing as expected
		if strings.Contains(errorStr, "task") && strings.Contains(errorStr, "not found") ||
			strings.Contains(errorStr, "operation failed") ||
			strings.Contains(output, "Error getting status") ||
			strings.Contains(output, "Task test-task-id not found") {
			klog.Infof("Script executed successfully (expected task not found error): %v", err)
			return true
		}

		// Check if we got XML output indicating the script ran
		if strings.Contains(output, "<?xml") || strings.Contains(output, "<o>") {
			klog.Infof("Received XML response from script - SSH working correctly")
			return true
		}

		// For any other error, assume it's a script execution issue but SSH is working
		klog.Infof("Script executed with error (SSH working): %v", err)
		return true
	}

	// If no error, that's also good - SSH is working
	klog.Infof("SSH test completed successfully")
	return true
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

// isESXi8OrNewer checks if the version is ESXi 8.0 or newer
func isESXi8OrNewer(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	return majorVersion >= 8
}

// enableSSHService enables the SSH service on ESXi host using vSphere API
func enableSSHService(vmwareClient Client, host *object.HostSystem, ctx context.Context) error {
	hostServiceSystem, err := host.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return fmt.Errorf("failed to get service system: %w", err)
	}

	err = hostServiceSystem.Start(ctx, "TSM-SSH")
	if err != nil {
		services, listErr := hostServiceSystem.Service(ctx)
		if listErr == nil {
			for _, service := range services {
				if service.Key == "TSM-SSH" {
					if service.Running {
						return nil
					}
					break
				}
			}
		}
		return fmt.Errorf("failed to start SSH service: %w", err)
	}

	return nil
}

// configureSSHFirewall enables the SSH firewall rule using vSphere API
func configureSSHFirewall(host *object.HostSystem, ctx context.Context) error {
	firewallSystem, err := host.ConfigManager().FirewallSystem(ctx)
	if err != nil {
		return fmt.Errorf("failed to get firewall system: %w", err)
	}

	err = firewallSystem.EnableRuleset(ctx, "sshServer")
	if err != nil {
		return fmt.Errorf("failed to enable SSH firewall ruleset: %w", err)
	}

	return nil
}

// suppressShellWarning disables the shell warning using vSphere API
func suppressShellWarning(host *object.HostSystem, ctx context.Context) error {
	configManager := host.ConfigManager()

	optionManager, err := configManager.OptionManager(ctx)
	if err != nil {
		return fmt.Errorf("failed to get option manager: %w", err)
	}

	option := []types.BaseOptionValue{
		&types.OptionValue{
			Key:   "/UserVars/SuppressShellWarning",
			Value: int32(1),
		},
	}

	err = optionManager.Update(ctx, option)
	if err != nil {
		return fmt.Errorf("failed to suppress shell warning: %w", err)
	}

	return nil
}

// installSSHKey installs the SSH public key on ESXi host
func installSSHKey(vmwareClient Client, host *object.HostSystem, publicKey string) error {
	ctx := context.Background()

	newCommand := []string{"system", "ssh", "key", "add", "-u", "root", "-k", publicKey}
	_, err := vmwareClient.RunEsxCommand(ctx, host, newCommand)
	if err == nil {
		return nil
	}

	return fmt.Errorf("automatic SSH key installation not supported for this ESXi version - manual setup required")
}

// ParseProgress extracts progress percentage from vmkfstools output
func ParseProgress(lastLine string) (uint, bool) {
	allMatches := progressPattern.FindAllStringSubmatch(lastLine, -1)
	if len(allMatches) > 0 {
		lastMatch := allMatches[len(allMatches)-1]
		if len(lastMatch) > 1 {
			if progress, err := strconv.Atoi(lastMatch[1]); err == nil {
				return uint(progress), true
			}
		}
	}
	return 0, false
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
