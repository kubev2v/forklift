package sshkeys

import (
	"fmt"
	"strings"
)

const (
	// SSHKeysSecretPrefix is the prefix used for SSH key secrets
	SSHKeysSecretPrefix = "offload-ssh-keys"
)

// SanitizeProviderName converts provider hostname to a valid Kubernetes resource name
func SanitizeProviderName(providerHostname string) string {
	sanitized := strings.ReplaceAll(providerHostname, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ToLower(sanitized)
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// GenerateSSHPrivateSecretName generates a secret name for SSH private key
func GenerateSSHPrivateSecretName(providerHostname string) string {
	return fmt.Sprintf("%s-%s-private", SSHKeysSecretPrefix, SanitizeProviderName(providerHostname))
}

// GenerateSSHPublicSecretName generates a secret name for SSH public key
func GenerateSSHPublicSecretName(providerHostname string) string {
	return fmt.Sprintf("%s-%s-public", SSHKeysSecretPrefix, SanitizeProviderName(providerHostname))
}
