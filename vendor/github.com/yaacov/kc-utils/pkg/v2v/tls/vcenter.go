package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// VCenterConfig builds a tls.Config for govmomi vCenter SDK connections.
func VCenterConfig(policy Policy) (*tls.Config, error) {
	switch policy.Mode {
	case ModeInsecure:
		return &tls.Config{InsecureSkipVerify: true}, nil
	case ModeSystemCA:
		return &tls.Config{}, nil
	case ModeCustomCA:
		pem, err := os.ReadFile(policy.CaBundlePath)
		if err != nil {
			return nil, fmt.Errorf("read CA bundle %s: %w", policy.CaBundlePath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse CA bundle %s: no valid PEM certificates", policy.CaBundlePath)
		}
		return &tls.Config{RootCAs: pool}, nil
	default:
		return nil, fmt.Errorf("unknown TLS mode %v", policy.Mode)
	}
}
