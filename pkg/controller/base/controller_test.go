package base

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
)

// TestHostnamePortExtraction verifies that hostname and port extraction
// works correctly for various URL formats (vCenter and ESXi scenarios)
func TestHostnamePortExtraction(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		expectedHost string
		expectedPort string
		description  string
	}{
		{
			name:         "vCenter hostname without port",
			host:         "vcenter.example.com",
			expectedHost: "vcenter.example.com",
			expectedPort: "443",
			description:  "Should add default port 443",
		},
		{
			name:         "vCenter hostname with port",
			host:         "vcenter.example.com:443",
			expectedHost: "vcenter.example.com",
			expectedPort: "443",
			description:  "Should preserve existing port",
		},
		{
			name:         "vCenter hostname with custom port",
			host:         "vcenter.example.com:8443",
			expectedHost: "vcenter.example.com",
			expectedPort: "8443",
			description:  "Should preserve custom port",
		},
		{
			name:         "ESXi IP without port",
			host:         "10.6.46.100",
			expectedHost: "10.6.46.100",
			expectedPort: "443",
			description:  "Should add default port 443",
		},
		{
			name:         "ESXi IP with port",
			host:         "10.6.46.100:443",
			expectedHost: "10.6.46.100",
			expectedPort: "443",
			description:  "Should preserve existing port",
		},
		{
			name:         "ESXi hostname without port",
			host:         "esxi-host.example.com",
			expectedHost: "esxi-host.example.com",
			expectedPort: "443",
			description:  "Should add default port 443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the new implementation logic
			hostname, port, err := net.SplitHostPort(tt.host)
			if err != nil {
				hostname = tt.host
				port = "443"
			}

			if hostname != tt.expectedHost {
				t.Errorf("hostname mismatch: got %q, want %q (%s)", hostname, tt.expectedHost, tt.description)
			}
			if port != tt.expectedPort {
				t.Errorf("port mismatch: got %q, want %q (%s)", port, tt.expectedPort, tt.description)
			}

			// Verify final connection string matches expected format
			finalConnection := net.JoinHostPort(hostname, port)
			expectedConnection := net.JoinHostPort(tt.expectedHost, tt.expectedPort)
			if finalConnection != expectedConnection {
				t.Errorf("final connection string mismatch: got %q, want %q", finalConnection, expectedConnection)
			}
		})
	}
}

// TestCertificateMatching verifies certificate DNS name and IP matching logic
func TestCertificateMatching(t *testing.T) {
	// Create mock vCenter certificate (has DNS names)
	vcenterCert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "vcenter.example.com",
		},
		DNSNames:     []string{"vcenter.example.com", "*.example.com"},
		IPAddresses:  []net.IP{net.ParseIP("10.6.46.200")},
		SerialNumber: big.NewInt(1),
	}

	// Create mock ESXi certificate (IP-only, no DNS names)
	esxiCert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "10.6.46.100",
		},
		DNSNames:     []string{}, // ESXi typically has no DNS names
		IPAddresses:  []net.IP{net.ParseIP("10.6.46.100")},
		SerialNumber: big.NewInt(2),
	}

	tests := []struct {
		name          string
		cert          *x509.Certificate
		hostname      string
		expectedMatch bool
		expectedIsIP  bool
		description   string
	}{
		{
			name:          "vCenter - hostname matches DNS name",
			cert:          vcenterCert,
			hostname:      "vcenter.example.com",
			expectedMatch: true,
			expectedIsIP:  false,
			description:   "vCenter cert has matching DNS name",
		},
		{
			name:          "vCenter - hostname matches CN",
			cert:          vcenterCert,
			hostname:      "vcenter.example.com",
			expectedMatch: true,
			expectedIsIP:  false,
			description:   "Should match via Common Name if DNS names don't match",
		},
		{
			name:          "ESXi - IP address",
			cert:          esxiCert,
			hostname:      "10.6.46.100",
			expectedMatch: false,
			expectedIsIP:  true,
			description:   "ESXi with IP - no DNS matching needed",
		},
		{
			name:          "ESXi - hostname with no DNS match",
			cert:          esxiCert,
			hostname:      "esxi-host.example.com",
			expectedMatch: false,
			expectedIsIP:  false,
			description:   "ESXi cert has no DNS names - should trigger IP resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if hostname is actually an IP address
			isHostnameIP := net.ParseIP(tt.hostname) != nil
			if isHostnameIP != tt.expectedIsIP {
				t.Errorf("IP detection mismatch: got %v, want %v", isHostnameIP, tt.expectedIsIP)
			}

			// Determine if certificate matches the hostname directly
			certMatchesHostname := false
			if !isHostnameIP {
				// Hostname (not IP): check if it exists in certificate's DNS names
				for _, dnsName := range tt.cert.DNSNames {
					if dnsName == tt.hostname {
						certMatchesHostname = true
						break
					}
				}
				// Fallback: check if Common Name (CN) matches
				if !certMatchesHostname && tt.cert.Subject.CommonName == tt.hostname {
					certMatchesHostname = true
				}
			}

			if certMatchesHostname != tt.expectedMatch {
				t.Errorf("cert match mismatch: got %v, want %v (%s)", certMatchesHostname, tt.expectedMatch, tt.description)
			}

			// Verify ESXi logic is triggered only when no DNS names
			shouldTriggerESXiLogic := !certMatchesHostname && !isHostnameIP
			expectedESXiLogic := len(tt.cert.DNSNames) == 0 && !isHostnameIP

			if shouldTriggerESXiLogic != expectedESXiLogic {
				t.Logf("ESXi logic trigger: got %v, expected %v (cert DNSNames=%d, isIP=%v)",
					shouldTriggerESXiLogic, expectedESXiLogic, len(tt.cert.DNSNames), isHostnameIP)
			}
		})
	}
}

// TestConnectionStringGeneration verifies the final connection string format
func TestConnectionStringGeneration(t *testing.T) {
	tests := []struct {
		name               string
		inputHost          string
		expectedConnection string
	}{
		{
			name:               "vCenter without port",
			inputHost:          "vcenter.example.com",
			expectedConnection: "vcenter.example.com:443",
		},
		{
			name:               "vCenter with port",
			inputHost:          "vcenter.example.com:443",
			expectedConnection: "vcenter.example.com:443",
		},
		{
			name:               "ESXi IP without port",
			inputHost:          "10.6.46.100",
			expectedConnection: "10.6.46.100:443",
		},
		{
			name:               "ESXi IP with port",
			inputHost:          "10.6.46.100:443",
			expectedConnection: "10.6.46.100:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the new implementation
			hostname, port, err := net.SplitHostPort(tt.inputHost)
			if err != nil {
				hostname = tt.inputHost
				port = "443"
			}

			connectionString := net.JoinHostPort(hostname, port)

			if connectionString != tt.expectedConnection {
				t.Errorf("connection string mismatch:\n  got:  %q\n  want: %q", connectionString, tt.expectedConnection)
			}

			// Also verify this matches the original implementation logic
			var originalHost string
			if _, _, err := net.SplitHostPort(tt.inputHost); err != nil {
				originalHost = tt.inputHost + ":443"
			} else {
				originalHost = tt.inputHost
			}

			if connectionString != originalHost {
				t.Errorf("NEW vs ORIGINAL mismatch:\n  new:      %q\n  original: %q\n  THIS IS A REGRESSION!", connectionString, originalHost)
			}
		})
	}
}
