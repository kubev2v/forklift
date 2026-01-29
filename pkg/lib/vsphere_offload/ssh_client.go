package vsphere_offload

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware"
	"github.com/vmware/govmomi/object"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

// SSHClient interface for SSH operations
type SSHClient interface {
	Connect(ctx context.Context, hostname, username string, privateKey []byte) error
	ExecuteCommand(datastore, sshCommand string, args ...string) (string, error)
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

// ExecuteCommand executes a command using the SSH_ORIGINAL_COMMAND pattern
// Uses structured format: DS=<datastore>;CMD=<operation> <args...>
// If datastore is empty, only tests connectivity without calling the wrapper
func (c *ESXiSSHClient) ExecuteCommand(datastore, sshCommand string, args ...string) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("SSH client not connected")
	}

	// Create a new session for this command
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Build the command part
	cmdPart := sshCommand
	if len(args) > 0 {
		cmdPart = fmt.Sprintf("%s %s", sshCommand, strings.Join(args, " "))
	}

	// Build structured command: DS=<datastore>;CMD=<command>
	// For connectivity tests, datastore can be empty
	fullCommand := fmt.Sprintf("DS=%s;CMD=%s", datastore, cmdPart)

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

func (c *ESXiSSHClient) Close() error {
	if c.sshClient != nil {
		err := c.sshClient.Close()
		c.sshClient = nil
		klog.Infof("Closed SSH connection to %s", c.hostname)
		return err
	}
	return nil
}

// EnableSSHAccess enables SSH service on ESXi host and provides manual SSH key installation instructions
func EnableSSHAccess(ctx context.Context, vmwareClient vmware.Client, host *object.HostSystem, privateKey, publicKey []byte, scriptPath string) error {
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

	// Use the shared restricted SSH command template
	restrictedPublicKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`,
		util.RestrictedSSHCommandTemplate, publicKeyStr)

	// Step 7: Test SSH connectivity first (using private key for authentication)
	// Pass empty datastore for connectivity test - the wrapper won't be called
	// Create a logger adapter from klog to logging.LevelLogger
	log := logging.WithName("ssh-setup")
	if util.TestSSHConnectivity(ctx, hostIP, privateKey, log) {
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

// getESXiVersion retrieves the ESXi version from the host
func getESXiVersion(vmwareClient vmware.Client, host *object.HostSystem, ctx context.Context) (string, error) {
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
