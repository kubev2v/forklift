package sshkeys

import (
	"errors"
	"fmt"

	"github.com/kubev2v/forklift/pkg/lib/logging"
)

const (
	// SSHKeysSecretPrefix is the prefix used for SSH key secrets
	SSHKeysSecretPrefix = "offload-ssh-keys"
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
