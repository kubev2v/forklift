package sshkeys

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	// SSHKeysSecretPrefix is the prefix used for SSH key secrets
	SSHKeysSecretPrefix = "offload-ssh-keys"
)

// SanitizeProviderName converts provider hostname to a valid Kubernetes secret name
// The result is validated to ensure it meets DNS1123Subdomain requirements (RFC 1123)
// Returns an error if the hostname cannot be sanitized to a valid Kubernetes secret name
func SanitizeProviderName(providerHostname string) (string, error) {
	if providerHostname == "" {
		return "", errors.New("provider hostname cannot be empty")
	}

	// Basic character sanitization for DNS1123Subdomain (allows dots)
	sanitized := strings.ReplaceAll(providerHostname, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ToLower(sanitized)
	// DNS1123Subdomain allows: lowercase alphanumeric, hyphens, and dots
	nonValidChars := regexp.MustCompile(`[^a-z0-9.-]`)
	sanitized = nonValidChars.ReplaceAllString(sanitized, "-")
	multipleDashes := regexp.MustCompile(`-+`)
	sanitized = multipleDashes.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-.") // Remove leading/trailing dashes and dots

	// Check if we have any valid characters left
	if sanitized == "" {
		return "", fmt.Errorf("provider hostname '%s' contains no valid characters for Kubernetes resource name", providerHostname)
	}

	// Ensure it starts and ends with alphanumeric character
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(sanitized) {
		sanitized = "h" + sanitized
	}
	if !regexp.MustCompile(`[a-z0-9]$`).MatchString(sanitized) {
		sanitized = sanitized + "0"
	}

	// Check if sanitized name will fit within Kubernetes DNS1123Subdomain limits (253 chars)
	// when combined with the secret name prefix and suffix
	// Private key: "offload-ssh-keys-{sanitized}-private" (25 + sanitized)
	// Public key: "offload-ssh-keys-{sanitized}-public" (24 + sanitized)
	// Using the more restrictive limit of 228 characters for sanitized name
	const maxSanitizedLength = 228
	if len(sanitized) > maxSanitizedLength {
		return "", fmt.Errorf("provider hostname '%s' results in a name that is too long (%d characters) for Kubernetes secret names (max %d characters after sanitization)", providerHostname, len(sanitized), maxSanitizedLength)
	}

	// Final validation using Kubernetes validation for DNS1123Subdomain
	errs := k8svalidation.IsDNS1123Subdomain(sanitized)
	if len(errs) > 0 {
		return "", fmt.Errorf("failed to create valid Kubernetes secret name from hostname '%s': %v", providerHostname, errs)
	}

	return sanitized, nil
}

// GenerateSSHPrivateSecretName generates a secret name for SSH private key
func GenerateSSHPrivateSecretName(providerHostname string) (string, error) {
	sanitized, err := SanitizeProviderName(providerHostname)
	if err != nil {
		return "", fmt.Errorf("failed to generate SSH private secret name: %w", err)
	}
	return fmt.Sprintf("%s-%s-private", SSHKeysSecretPrefix, sanitized), nil
}

// GenerateSSHPublicSecretName generates a secret name for SSH public key
func GenerateSSHPublicSecretName(providerHostname string) (string, error) {
	sanitized, err := SanitizeProviderName(providerHostname)
	if err != nil {
		return "", fmt.Errorf("failed to generate SSH public secret name: %w", err)
	}
	return fmt.Sprintf("%s-%s-public", SSHKeysSecretPrefix, sanitized), nil
}
