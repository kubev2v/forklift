package base

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestClientURL_UsesConfiguredScheme(t *testing.T) {
	tests := []struct {
		name   string
		scheme string
		host   string
		port   int
		path   string
		want   string
	}{
		{
			name:   "http scheme",
			scheme: "http",
			host:   "api.example.com",
			port:   8080,
			path:   "/api/v1/vms",
			want:   "http://api.example.com:8080/api/v1/vms",
		},
		{
			name:   "https scheme",
			scheme: "https",
			host:   "api.example.com",
			port:   8443,
			path:   "/api/v1/providers",
			want:   "https://api.example.com:8443/api/v1/providers",
		},
		{
			name:   "rewrites localhost selfLink host",
			scheme: "https",
			host:   "forklift-inventory.openshift-mtv.svc.cluster.local",
			port:   8443,
			path:   "https://localhost:8443/providers/vsphere/uid/vms/vm-1",
			want:   "https://forklift-inventory.openshift-mtv.svc.cluster.local:8443/providers/vsphere/uid/vms/vm-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origHost, origPort, origScheme := Settings.Host, Settings.Inventory.Port, Settings.Scheme
			t.Cleanup(func() {
				Settings.Host = origHost
				Settings.Inventory.Port = origPort
				Settings.Scheme = origScheme
			})

			Settings.Host = tt.host
			Settings.Inventory.Port = tt.port
			Settings.Scheme = tt.scheme

			c := &RestClient{Host: ""}
			got := c.url(tt.path)

			if got != tt.want {
				t.Fatalf("expected URL %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRestClientBuildTransport_VerifiesWithConfiguredCA(t *testing.T) {
	origCA, origDev := Settings.Inventory.TLS.CA, Settings.Development
	t.Cleanup(func() {
		Settings.Inventory.TLS.CA = origCA
		Settings.Development = origDev
	})

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}

	Settings.Inventory.TLS.CA = caPath
	Settings.Development = false

	c := &RestClient{}
	if err := c.buildTransport(); err != nil {
		t.Fatalf("buildTransport: %v", err)
	}
	transport, ok := c.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatalf("expected http.Transport with TLSClientConfig, got %#v", c.Transport)
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify must be false when a CA pool is configured")
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("RootCAs must be set when Inventory.TLS.CA is configured")
	}
}

func TestRestClientBuildTransport_DevelopmentWithoutCASkipsVerify(t *testing.T) {
	origCA, origDev := Settings.Inventory.TLS.CA, Settings.Development
	t.Cleanup(func() {
		Settings.Inventory.TLS.CA = origCA
		Settings.Development = origDev
	})

	Settings.Inventory.TLS.CA = ""
	Settings.Development = true

	c := &RestClient{}
	if err := c.buildTransport(); err != nil {
		t.Fatalf("buildTransport: %v", err)
	}
	transport, ok := c.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatalf("expected http.Transport with TLSClientConfig, got %#v", c.Transport)
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("development without CA should set InsecureSkipVerify")
	}
}
