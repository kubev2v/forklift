package announce

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubev2v/forklift/pkg/nbd-container/runner"
)

func TestAnnounceRequiresClientCert(t *testing.T) {
	certsDir := t.TempDir()
	caCert, caKey := writeCA(t, certsDir)
	writeLeaf(t, certsDir, "server", caCert, caKey, x509.ExtKeyUsageServerAuth, "localhost")
	clientTLS := clientCert(t, certsDir, caCert, caKey)

	exports := []runner.Export{{WWID: "wwn-abc", Port: 10891, Device: "/dev/sdb"}}
	srv, err := New("127.0.0.1:0", certsDir, exports)
	if err != nil {
		t.Fatal(err)
	}

	// Listen on an ephemeral port and serve in the background.
	ln, err := tls.Listen("tcp", "127.0.0.1:0", srv.http.TLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() { _ = srv.http.Serve(ln) }()
	addr := ln.Addr().String()

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// With a valid client cert: expect the export list back.
	withCert := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{clientTLS},
		ServerName:   "localhost",
	}}}
	resp, err := withCert.Get("https://" + addr + "/disks")
	if err != nil {
		t.Fatalf("request with client cert failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var got []runner.Export
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(got) != 1 || got[0] != exports[0] {
		t.Errorf("got %+v, want %+v", got, exports)
	}

	// Without a client cert: the handshake must be rejected.
	noCert := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:    caPool,
		ServerName: "localhost",
	}}}
	rejected, err := noCert.Get("https://" + addr + "/disks")
	if err == nil {
		_ = rejected.Body.Close()
		t.Error("request without client cert succeeded; mutual TLS not enforced")
	}
}

// --- test cert helpers (generated in-process; no external tools) ---

func writeCA(t *testing.T, dir string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, filepath.Join(dir, CACert), "CERTIFICATE", der)
	return cert, key
}

func writeLeaf(t *testing.T, dir, name string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, eku x509.ExtKeyUsage, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{eku},
		DNSNames:     []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, filepath.Join(dir, name+"-cert.pem"), "CERTIFICATE", der)
	writePEM(t, filepath.Join(dir, name+"-key.pem"), "PRIVATE KEY", keyDER)

	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, name+"-cert.pem"), filepath.Join(dir, name+"-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func clientCert(t *testing.T, dir string, ca *x509.Certificate, caKey *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()
	return writeLeaf(t, dir, "client", ca, caKey, x509.ExtKeyUsageClientAuth, "client")
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatal(err)
	}
}
