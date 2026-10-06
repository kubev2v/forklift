package tlsconfig

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/kubev2v/vm-migration-detective/pkg/types"
	"github.com/sirupsen/logrus"
)

// Config represents validated TLS configuration
type Config struct {
	Insecure            bool           // Whether to skip TLS verification
	RootCAs             *x509.CertPool // CA certificate pool for verification
	RootCAPath          string         // CA certificate bundle path for clients that load PEM files directly
	Thumbprint          string         // Certificate thumbprint for pinning
	IsDeprecatedDefault bool           // Whether this is the deprecated default (no config)
}

// FromCredentials creates a TLS config from Credentials
// Returns configured TLS settings or deprecated default with warnings
func FromCredentials(creds types.Credentials, logger *logrus.Logger) (*Config, error) {
	// Explicit insecure mode (user acknowledged the risk)
	if creds.TLSInsecure {
		if logger != nil {
			logger.Warn("TLS verification explicitly DISABLED (TLSInsecure=true)")
			logger.Warn("This should only be used for testing/development")
			logger.Warn("vCenter credentials are vulnerable to MITM attacks")
		}
		return &Config{
			Insecure:            true,
			IsDeprecatedDefault: false,
		}, nil
	}

	// Thumbprint pinning (takes precedence over CA bundle)
	if creds.TLSThumbprint != "" {
		normalized, err := NormalizeThumbprint(creds.TLSThumbprint)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS thumbprint: %w", err)
		}
		return &Config{
			Insecure:            false,
			Thumbprint:          normalized,
			IsDeprecatedDefault: false,
		}, nil
	}

	// CA bundle verification
	if creds.TLSCACert != "" {
		pool, err := LoadCACertPool(creds.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
		return &Config{
			Insecure:            false,
			RootCAs:             pool,
			RootCAPath:          creds.TLSCACert,
			IsDeprecatedDefault: false,
		}, nil
	}

	// No TLS configuration provided - deprecated default
	// Log loud warning and return insecure config for backward compatibility
	LogDeprecationWarning(logger)
	return &Config{
		Insecure:            true,
		IsDeprecatedDefault: true,
	}, nil
}

// LoadCACertPool loads CA certificates from a PEM file
func LoadCACertPool(certPath string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate file %s: %w", certPath, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s: invalid PEM format", certPath)
	}

	return pool, nil
}

// NormalizeThumbprint validates and normalizes a thumbprint
// Accepts: "AA:BB:CC...", "aabbcc...", or "AA BB CC..."
// Returns: uppercase colon-separated format
func NormalizeThumbprint(thumbprint string) (string, error) {
	// Remove spaces and colons
	cleaned := strings.ReplaceAll(thumbprint, ":", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Validate hex string length (SHA-1 = 20 bytes = 40 hex chars)
	if len(cleaned) != 40 {
		return "", fmt.Errorf("thumbprint must be 40 hex characters (SHA-1), got %d", len(cleaned))
	}

	// Validate hex
	if _, err := hex.DecodeString(cleaned); err != nil {
		return "", fmt.Errorf("thumbprint contains invalid hex characters: %w", err)
	}

	// Format as colon-separated uppercase
	normalized := strings.ToUpper(cleaned)
	var formatted strings.Builder
	for i := 0; i < len(normalized); i += 2 {
		if i > 0 {
			formatted.WriteString(":")
		}
		formatted.WriteString(normalized[i : i+2])
	}

	return formatted.String(), nil
}

// ComputeThumbprint computes the VMware-compatible SHA-1 thumbprint of a certificate
// Returns VMware format (colon-separated uppercase hex)
func ComputeThumbprint(certDER []byte) string {
	hash := sha1.Sum(certDER)
	hexStr := hex.EncodeToString(hash[:])

	var formatted strings.Builder
	for i := 0; i < len(hexStr); i += 2 {
		if i > 0 {
			formatted.WriteString(":")
		}
		formatted.WriteString(strings.ToUpper(hexStr[i : i+2]))
	}

	return formatted.String()
}

// GetVCenterThumbprint retrieves the thumbprint from vCenter
// Uses the provided TLS config for the initial connection
func GetVCenterThumbprint(vcenterHost string, tlsConfig *Config) (string, error) {
	var dialTLSConfig *tls.Config

	if tlsConfig.Insecure {
		// Insecure mode - skip verification to retrieve cert
		dialTLSConfig = &tls.Config{InsecureSkipVerify: true}
	} else if tlsConfig.RootCAs != nil {
		// Use CA bundle for verification
		dialTLSConfig = &tls.Config{
			RootCAs:    tlsConfig.RootCAs,
			ServerName: vcenterHost,
		}
	} else if tlsConfig.Thumbprint != "" {
		// For thumbprint mode, verify using custom callback
		dialTLSConfig = &tls.Config{
			InsecureSkipVerify: true, // We'll verify thumbprint manually
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return fmt.Errorf("no certificates presented")
				}

				actualThumbprint := ComputeThumbprint(rawCerts[0])
				if actualThumbprint != tlsConfig.Thumbprint {
					return fmt.Errorf("certificate thumbprint mismatch: expected %s, got %s (possible MITM attack)",
						tlsConfig.Thumbprint, actualThumbprint)
				}
				return nil
			},
		}
	} else {
		return "", fmt.Errorf("no valid TLS configuration")
	}

	conn, err := tls.Dial("tcp", net.JoinHostPort(vcenterHost, "443"), dialTLSConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect to vCenter: %w", err)
	}
	defer func() { _ = conn.Close() }()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates found")
	}

	return ComputeThumbprint(certs[0].Raw), nil
}

// ForGovmomi returns whether to use insecure mode for govmomi
// govmomi only supports insecure bool, not custom TLS config
func (c *Config) ForGovmomi() bool {
	return c.Insecure
}

// ForNBDKit returns the thumbprint parameter for nbdkit-vddk
// Returns empty string if no thumbprint available
func (c *Config) ForNBDKit() string {
	if c.Insecure {
		return "" // No thumbprint in insecure mode
	}
	return c.Thumbprint
}

// ForVirtV2V returns the sslVerify URL parameter for virt-v2v vpx:// URLs
// Returns "no_verify=1" for insecure, "cacert=PATH" for CA
func (c *Config) ForVirtV2V(caPath string) string {
	if c.Insecure {
		return "no_verify=1"
	}

	// virt-v2v supports cacert parameter
	if c.RootCAs != nil && caPath != "" {
		return fmt.Sprintf("cacert=%s", caPath)
	}

	// For thumbprint mode, virt-v2v doesn't support thumbprint pinning directly
	// Best option is to use no_verify and rely on nbdkit thumbprint verification
	return "no_verify=1"
}

// LogDeprecationWarning logs a multi-line security warning about insecure mode
// Outputs to both stderr (always) and logger (if not nil)
func LogDeprecationWarning(logger *logrus.Logger) {
	warning := `════════════════════════════════════════════════════════════════════════
SECURITY WARNING: TLS verification is DISABLED for vCenter connections!
════════════════════════════════════════════════════════════════════════

This configuration is DEPRECATED and will be REQUIRED in v1.0.0
Your vCenter credentials are vulnerable to MITM attacks.

Please configure TLS verification in Credentials:
  • TLSCACert: "/path/to/ca-bundle.crt" (recommended for production)
  • TLSThumbprint: "AA:BB:CC:..." (certificate pinning)
  • TLSInsecure: true (explicit opt-in for testing only)

Documentation: https://github.com/kubev2v/vm-migration-detective#tls-configuration
════════════════════════════════════════════════════════════════════════`

	// Always output to stderr
	fmt.Fprintln(os.Stderr, warning)

	// Also log if logger is available
	if logger != nil {
		for _, line := range strings.Split(warning, "\n") {
			logger.Warn(line)
		}
	}
}
