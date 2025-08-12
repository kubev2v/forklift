package sshkeys

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecureScriptVersion defines the version of the secure vmkfstools wrapper script
// This should be kept in sync with the actual script version
const SecureScriptVersion = "1.0.0"

const (
	// SSHKeysSecretPrefix is the prefix used for SSH key secrets
	SSHKeysSecretPrefix = "offload-ssh-keys"
)

// SanitizeProviderName converts provider name to a valid Kubernetes resource name
func SanitizeProviderName(providerName string) string {
	sanitized := strings.ReplaceAll(providerName, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ToLower(sanitized)
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// GenerateSSHPrivateSecretName generates a secret name for SSH private key
func GenerateSSHPrivateSecretName(providerName string) string {
	return fmt.Sprintf("%s-%s-private", SSHKeysSecretPrefix, SanitizeProviderName(providerName))
}

// GenerateSSHPublicSecretName generates a secret name for SSH public key
func GenerateSSHPublicSecretName(providerName string) string {
	return fmt.Sprintf("%s-%s-public", SSHKeysSecretPrefix, SanitizeProviderName(providerName))
}

func TestSSHConnectivity(hostname string, privateKey []byte) bool {
	// Parse the private key
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return false
	}

	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Connect to the SSH server
	conn, err := ssh.Dial("tcp", net.JoinHostPort(hostname, "22"), config)
	if err != nil {
		return false
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return false
	}
	defer session.Close()

	// Test with the restricted command pattern
	testCommand := "status test-task-id"
	output, err := session.CombinedOutput(testCommand)
	outputStr := string(output)

	return EvaluateSSHTestResult(outputStr, err)
}

// EvaluateSSHTestResult analyzes SSH command output and error to determine if connectivity is working.
// This function contains the shared logic for determining SSH connectivity success.
func EvaluateSSHTestResult(output string, err error) bool {
	if err != nil {
		errorStr := strings.ToLower(err.Error())
		outputLower := strings.ToLower(output)

		// These errors indicate SSH key/connectivity problems - FAILURE
		if strings.Contains(errorStr, "permission denied") ||
			strings.Contains(errorStr, "connection refused") ||
			strings.Contains(errorStr, "host key verification failed") ||
			strings.Contains(errorStr, "authentication failed") {
			return false
		}

		// These conditions indicate SSH connectivity is working - SUCCESS
		if strings.Contains(errorStr, "task test-task-id not found") ||
			strings.Contains(outputLower, "task test-task-id not found") ||
			strings.Contains(outputLower, "no such file or directory") ||
			strings.Contains(errorStr, "no such file or directory") {
			return true
		}

		return true
	} else {
		outputLower := strings.ToLower(output)
		return strings.Contains(outputLower, "task test-task-id not found")
	}
}

// GetSSHPrivateKey retrieves the SSH private key for a provider using the provider name pattern
func GetSSHPrivateKey(k8sClient client.Client, providerName, namespace string) ([]byte, error) {
	// Use provider name based secret naming pattern
	privateSecretName := GenerateSSHPrivateSecretName(providerName)

	// Get the private key secret
	secret := &core.Secret{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      privateSecretName,
	}
	err := k8sClient.Get(context.Background(), key, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH private key secret %s: %w", privateSecretName, err)
	}

	// Extract private key using the exact field name from volume populator
	privateKeyBytes, hasPrivate := secret.Data["private-key"]
	if !hasPrivate {
		return nil, fmt.Errorf("SSH private key not found in secret %s", privateSecretName)
	}

	return privateKeyBytes, nil
}
