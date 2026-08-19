// Package tls provides vCenter SDK TLS policy (insecure, custom CA, system CA).
// ESXi NFC disk downloads use govmomi lease thumbprints in pkg/copy, not this package.
package tls

import (
	"fmt"
	"os"

	"github.com/yaacov/kc-utils/pkg/v2v/config"
)

// Mode selects how the vCenter SDK verifies TLS.
type Mode int

const (
	ModeInsecure Mode = iota
	ModeCustomCA
	ModeSystemCA
)

// Policy is the resolved vCenter TLS mode and PEM bundle path.
type Policy struct {
	Mode         Mode
	CaBundlePath string // PEM file for ModeCustomCA only
}

// ForkliftTLS resolves TLS for kc-v2v using Forklift conversion-pod signals only.
//
//  1. no_verify=1 in V2V_libvirtURL → ModeInsecure
//  2. /etc/secret/cacert mounted → ModeCustomCA
//  3. else → ModeSystemCA
func ForkliftTLS(insecureFromURL bool) (Policy, error) {
	if insecureFromURL {
		return Policy{Mode: ModeInsecure}, nil
	}
	if fileExists(config.DefaultCaCert) {
		return Policy{Mode: ModeCustomCA, CaBundlePath: config.DefaultCaCert}, nil
	}
	return Policy{Mode: ModeSystemCA}, nil
}

// CopyTLS resolves TLS for kc-copy from insecure and ca_cert fields only.
//
//  1. insecure → ModeInsecure
//  2. ca_cert set → ModeCustomCA (error if file missing)
//  3. else → ModeSystemCA
func CopyTLS(insecure bool, caCert string) (Policy, error) {
	if insecure {
		return Policy{Mode: ModeInsecure}, nil
	}
	if caCert != "" {
		if !fileExists(caCert) {
			return Policy{}, fmt.Errorf("CA cert file not found: %s", caCert)
		}
		return Policy{Mode: ModeCustomCA, CaBundlePath: caCert}, nil
	}
	return Policy{Mode: ModeSystemCA}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
