package vmware

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/vmware/govmomi/object"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

// SSHClient interface for SSH operations
type SSHClient interface {
	Connect(ctx context.Context, hostname, username string, privateKey []byte) error
	ExecuteCommand(ctx context.Context, datastore, sshCommand string, args ...string) (string, error)
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

	return c.connect(ctx)
}

// connect establishes (or re-establishes) the underlying SSH connection using
// the hostname, username, and privateKey already stored on the receiver.
func (c *ESXiSSHClient) connect(ctx context.Context) error {
	// Parse the private key
	signer, err := ssh.ParsePrivateKey(c.privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User: c.username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Establish TCP connection honoring context cancellation/deadline
	addr := net.JoinHostPort(c.hostname, "22")
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Enable TCP keepalive to prevent connection drops during long operations
	tcpConn, ok := netConn.(*net.TCPConn)
	if !ok {
		_ = netConn.Close()
		return fmt.Errorf("connection is not a TCP connection")
	}
	if err := tcpConn.SetKeepAlive(true); err != nil {
		_ = netConn.Close()
		return fmt.Errorf("failed to enable TCP keepalive: %w", err)
	}
	if err := tcpConn.SetKeepAlivePeriod(15 * time.Second); err != nil {
		_ = netConn.Close()
		return fmt.Errorf("failed to set TCP keepalive period: %w", err)
	}

	// Ensure the SSH handshake also respects the context deadline
	if deadline, ok := ctx.Deadline(); ok {
		if err := netConn.SetDeadline(deadline); err != nil {
			_ = netConn.Close()
			return fmt.Errorf("failed to set connection deadline: %w", err)
		}
	}

	// Perform SSH handshake on the established net.Conn
	cc, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		_ = netConn.Close()
		return fmt.Errorf("failed to establish SSH client connection: %w", err)
	}
	c.sshClient = ssh.NewClient(cc, chans, reqs)
	klog.FromContext(ctx).WithName("ssh").Info("connected to SSH server", "host", c.hostname)
	return nil
}

// ExecuteCommand executes a command using the SSH_ORIGINAL_COMMAND pattern
// Uses structured format: DS=<datastore>;CMD=<operation> <args...>
// If datastore is empty, only tests connectivity without calling the wrapper
func (c *ESXiSSHClient) ExecuteCommand(ctx context.Context, datastore, sshCommand string, args ...string) (string, error) {
	log := klog.FromContext(ctx).WithName("ssh")

	// Create a new session for this command with retry logic.
	// If session creation fails (e.g. because the underlying TCP connection
	// dropped), attempt to reconnect the SSH client before retrying.
	var session *ssh.Session
	var sessionErr error
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if c.sshClient == nil {
			// Client is nil — either never connected or previous reconnection
			// failed.  Treat this the same as a session-creation failure so
			// the reconnection logic below gets a chance to run.
			sessionErr = fmt.Errorf("SSH client not connected")
		} else {
			session, sessionErr = c.sshClient.NewSession()
			if sessionErr == nil {
				break
			}
		}
		if i < maxRetries-1 {
			log.Info("SSH session creation failed, reconnecting", "attempt", i+1, "error", sessionErr)
			// Close the dead connection (best-effort)
			if c.sshClient != nil {
				_ = c.sshClient.Close()
				c.sshClient = nil
			}
			// Exponential backoff: 1s, 2s
			backoffDuration := time.Duration(1<<uint(i)) * time.Second
			select {
			case <-time.After(backoffDuration):
			case <-ctx.Done():
				return "", fmt.Errorf("SSH reconnection cancelled during backoff: %w", ctx.Err())
			}
			// Attempt to re-establish the SSH connection
			reconnCtx, reconnCancel := context.WithTimeout(ctx, 30*time.Second)
			if reconnErr := c.connect(reconnCtx); reconnErr != nil {
				reconnCancel()
				log.Info("SSH reconnection failed", "attempt", i+1, "error", reconnErr)
				continue
			}
			reconnCancel()
			log.Info("SSH reconnected successfully", "attempt", i+1)
		}
	}
	if sessionErr != nil {
		return "", fmt.Errorf("failed to create SSH session after %d attempts: %w", maxRetries, sessionErr)
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

	log.V(2).Info("executing SSH command", "command", fullCommand)

	// Create a context with timeout for the command execution
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
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
	case <-runCtx.Done():
		// Command timed out, try to close the session
		session.Close()
		return "", fmt.Errorf("SSH command timed out after 60 seconds: %s", fullCommand)
	}

	outputStr := string(output)

	if cmdErr != nil {
		log.Info("SSH command failed", "command", fullCommand, "output", outputStr, "err", cmdErr)
		return outputStr, cmdErr
	}

	log.V(2).Info("SSH command succeeded", "command", fullCommand, "output", outputStr)
	return outputStr, nil
}

func (c *ESXiSSHClient) Close() error {
	if c.sshClient != nil {
		err := c.sshClient.Close()
		c.sshClient = nil
		// No ctx available at Close; use base logger so we still show "ssh"
		klog.Background().WithName("copy-offload").WithName("ssh").Info("SSH connection closed", "host", c.hostname)
		return err
	}
	return nil
}

// EnableSSHAccess enables SSH service on ESXi host and provides manual SSH key installation instructions
func EnableSSHAccess(ctx context.Context, vmwareClient Client, host *object.HostSystem, privateKey, publicKey []byte, scriptPath string) error {
	publicKeyStr := strings.TrimSpace(string(publicKey))
	log := klog.Background().WithName("copy-offload").WithName("ssh")

	log.Info("enabling SSH access on ESXi host", "host", host.Name())
	ctx = klog.NewContext(ctx, log)

	hostIP, err := GetHostIPAddress(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get host IP address: %w", err)
	}

	version, err := getESXiVersion(vmwareClient, host, ctx)
	if err != nil {
		return fmt.Errorf("failed to get ESXi version: %w", err)
	}
	log.Info("ESXi version detected", "version", version)

	restrictedPublicKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`,
		util.RestrictedSSHCommandTemplate, publicKeyStr)

	sshSetupLog := logging.WithName("ssh-setup")
	if util.TestSSHConnectivity(ctx, hostIP, privateKey, sshSetupLog) {
		log.Info("SSH connectivity test passed, keys already configured")
		return nil
	}

	instructions := fmt.Sprintf(`Manual SSH key installation required. Add this line to /etc/ssh/keys-root/authorized_keys on the ESXi host:

  %s

The template extracts datastore from commands (DS=<datastore>;CMD=<command>) and executes: /vmfs/volumes/$DS/secure-vmkfstools-wrapper

Steps:
1. SSH to the ESXi host: ssh root@%s
2. Edit: vi /etc/ssh/keys-root/authorized_keys
3. Add the line above, save and exit
4. Restart the operation`, restrictedPublicKey, hostIP)
	log.Error(fmt.Errorf("manual SSH key configuration required"), "SSH key setup", "instructions", instructions)
	return fmt.Errorf("manual SSH key configuration required for ESXi %s - see logs for instructions", version)
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
