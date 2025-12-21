package util

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	"golang.org/x/crypto/ssh"
)

const (
	// SSHKeysSecretPrefix is the prefix used for SSH key secrets
	SSHKeysSecretPrefix = "offload-ssh-keys"

	// RestrictedSSHCommandTemplate is the inline shell command used in SSH authorized_keys
	// to restrict SSH access and route commands to the Python wrapper based on datastore.
	// Format: DS=<datastore>;UUID=<uuid>;CMD=<operation> <args...>
	// When DS is empty, it returns a simple success response for connectivity testing without calling the wrapper.
	// UUID is used to identify the specific script file: secure-vmkfstools-wrapper-{UUID}.py
	RestrictedSSHCommandTemplate = `sh -c 'DS=$(echo \"$SSH_ORIGINAL_COMMAND\" | sed -n \"s/^DS=\\([^;]*\\);.*/\\1/p\"); UUID=$(echo \"$SSH_ORIGINAL_COMMAND\" | sed -n \"s/^DS=[^;]*;UUID=\\([^;]*\\);.*/\\1/p\"); CMD=$(echo \"$SSH_ORIGINAL_COMMAND\" | sed -n \"s/^DS=[^;]*;UUID=[^;]*;CMD=\\(.*\\)/\\1/p\"); if [ -z \"$DS\" ]; then echo \"SSH_OK\"; else SSH_ORIGINAL_COMMAND=\"$CMD\" exec python /vmfs/volumes/$DS/secure-vmkfstools-wrapper-$UUID.py; fi'`
)

// SanitizeProviderName converts provider name to a valid Kubernetes secret name
// If the provider name is too long, it will be truncated to fit within Kubernetes limits
// and a warning will be logged
func SanitizeProviderName(providerName string) (string, error) {
	if providerName == "" {
		return "", errors.New("provider name cannot be empty")
	}

	// Check if name will fit within Kubernetes DNS1123Subdomain limits (253 chars)
	// when combined with the secret name prefix and suffix
	// Both private and public keys use 25 characters overhead: "offload-ssh-keys-{name}-private"
	// Maximum provider name length: 253 - 25 = 228 characters
	const maxProviderNameLength = 228
	if len(providerName) > maxProviderNameLength {
		log := logging.WithName("sshkeys")
		log.Info("Provider name too long, truncating",
			"originalName", providerName,
			"originalLength", len(providerName),
			"maxLength", maxProviderNameLength)
		providerName = providerName[:maxProviderNameLength]
	}

	return providerName, nil
}

// GenerateSSHPrivateSecretName generates a secret name for SSH private key
func GenerateSSHPrivateSecretName(providerName string) (string, error) {
	sanitized, err := SanitizeProviderName(providerName)
	if err != nil {
		return "", fmt.Errorf("failed to generate SSH private secret name: %w", err)
	}
	return fmt.Sprintf("%s-%s-private", SSHKeysSecretPrefix, sanitized), nil
}

// GenerateSSHPublicSecretName generates a secret name for SSH public key
func GenerateSSHPublicSecretName(providerName string) (string, error) {
	sanitized, err := SanitizeProviderName(providerName)
	if err != nil {
		return "", fmt.Errorf("failed to generate SSH public secret name: %w", err)
	}
	return fmt.Sprintf("%s-%s-public", SSHKeysSecretPrefix, sanitized), nil
}

// TestSSHConnectivity tests if we can connect via SSH and execute a restricted command.
// It takes a context, hostIP, privateKey, optional testDatastore, and a logger.
// If testDatastore is empty, it performs a simple connectivity test expecting "SSH_OK" response.
// If testDatastore is provided, it will try to call the shell wrapper on that datastore.
// Returns true if SSH connectivity is working, false otherwise.
func TestSSHConnectivity(ctx context.Context, hostIP string, privateKey []byte, log logging.LevelLogger) bool {
	log.V(3).Info("Testing SSH connectivity to host", "hostIP", hostIP)

	// Create context with timeout for connectivity testing to avoid indefinite hangs
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Parse the private key
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		log.V(2).Info("SSH connectivity test failed to parse private key", "hostIP", hostIP, "error", err)
		return false
	}

	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Establish TCP connection honoring context cancellation/deadline
	addr := net.JoinHostPort(hostIP, "22")
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(testCtx, "tcp", addr)
	if err != nil {
		log.V(2).Info("SSH connectivity test failed to connect", "hostIP", hostIP, "error", err)
		return false
	}

	// Ensure the SSH handshake also respects the context deadline
	if deadline, ok := testCtx.Deadline(); ok {
		_ = netConn.SetDeadline(deadline)
	}

	// Perform SSH handshake on the established net.Conn
	cc, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		_ = netConn.Close()
		log.V(2).Info("SSH connectivity test failed to establish SSH client connection", "hostIP", hostIP, "error", err)
		return false
	}
	sshClient := ssh.NewClient(cc, chans, reqs)
	defer sshClient.Close()

	log.V(3).Info("Connected to SSH server", "hostIP", hostIP)

	// Try to execute a simple test command using the structured format
	// Format: DS=<datastore>;UUID=<uuid>;CMD=status test-task-id
	// When DS is empty, the shell wrapper will return "SSH_OK" without calling Python
	session, err := sshClient.NewSession()
	if err != nil {
		log.V(2).Info("SSH connectivity test failed to create session", "hostIP", hostIP, "error", err)
		return false
	}
	defer session.Close()

	// Execute the status command with the provided datastore (or empty for connectivity test mode)
	// Format: DS=<datastore>;UUID=<uuid>;CMD=<command>
	// For connectivity test, DS and UUID are empty
	testCommand := "DS=;UUID=;CMD=status test-task-id"
	output, err := session.CombinedOutput(testCommand)

	log.V(3).Info("SSH test command output", "hostIP", hostIP, "command", testCommand, "output", string(output), "error", err)

	// Check for expected responses that indicate SSH is working
	outputStr := strings.TrimSpace(string(output))

	// Check for simple connectivity test response
	if outputStr == "SSH_OK" {
		log.V(2).Info("Received connectivity test response - SSH key configured correctly", "hostIP", hostIP)
		return true
	}

	return false
}
