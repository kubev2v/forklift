package announce

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubev2v/forklift/pkg/nbd-container/runner"
)

// The client talks to the real server over a real handshake, because what is
// worth testing here is the agreement between the two: the wire format, and the
// name the certificate is verified against.
func TestClientDisks(t *testing.T) {
	t.Run("the exports the server announces come back whole", func(t *testing.T) {
		exports := []runner.Export{
			{WWID: "wwn-abc", Port: 10809, Device: "/dev/sdb"},
			{WWID: "wwn-def", Port: 10810, Device: "/dev/sdc"},
		}
		dir := t.TempDir()
		addr := serveAnnounce(t, dir, exports)

		got, err := announceClient(t, dir).Disks(context.TODO(), addr)

		if err != nil {
			t.Fatalf("Disks: %v", err)
		}
		if len(got) != len(exports) {
			t.Fatalf("got %d exports, want %d: %+v", len(got), len(exports), got)
		}
		for i := range exports {
			if got[i] != exports[i] {
				t.Errorf("export %d = %+v, want %+v", i, got[i], exports[i])
			}
		}
	})

	// The orchestrator answers with an empty list while it is still starting
	// containers, so this is the normal early reply and not an error.
	t.Run("a server announcing nothing yet is not an error", func(t *testing.T) {
		dir := t.TempDir()
		addr := serveAnnounce(t, dir, nil)

		got, err := announceClient(t, dir).Disks(context.TODO(), addr)

		if err != nil {
			t.Fatalf("Disks: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %+v, want no exports", got)
		}
	})
}

// The appliance's address is not known when its certificate is signed, so the
// certificate names ServerName and the client verifies against that. A client
// that accepted whatever answered at the address would be trusting an
// unauthenticated export list, which is where a migration reads its disks from.
func TestClientVerifiesTheServer(t *testing.T) {
	t.Run("a server named something else is rejected", func(t *testing.T) {
		dir := t.TempDir()
		ca, caKey := writeCA(t, dir)
		writeLeaf(t, dir, "server", ca, caKey, x509.ExtKeyUsageServerAuth, "localhost")
		clientCert(t, dir, ca, caKey)
		addr := serve(t, dir, nil)

		_, err := announceClient(t, dir).Disks(context.TODO(), addr)

		if err == nil {
			t.Error("a server certificate issued for another name was accepted")
		}
	})

	t.Run("a server signed by another CA is rejected", func(t *testing.T) {
		dir := t.TempDir()
		ca, caKey := writeCA(t, dir)
		clientCert(t, dir, ca, caKey)
		// Signed by a CA the client has never heard of, while the CA in the
		// directory -- the one the client trusts, and the one the server
		// verifies clients with -- is left alone.
		otherCA, otherKey := writeCA(t, t.TempDir())
		writeLeaf(t, dir, "server", otherCA, otherKey, x509.ExtKeyUsageServerAuth, ServerName)
		addr := serve(t, dir, nil)

		_, err := announceClient(t, dir).Disks(context.TODO(), addr)

		if err == nil {
			t.Error("a server certificate from an unrelated CA was accepted")
		}
	})
}

// The client half of mutual TLS: the appliance is the one deciding, but a
// client built from material the appliance will not accept has to fail here
// rather than silently return nothing.
func TestClientPresentsItsCertificate(t *testing.T) {
	dir := t.TempDir()
	ca, caKey := writeCA(t, dir)
	writeLeaf(t, dir, "server", ca, caKey, x509.ExtKeyUsageServerAuth, ServerName)
	otherCA, otherKey := writeCA(t, t.TempDir())
	writeLeaf(t, dir, "client", otherCA, otherKey, x509.ExtKeyUsageClientAuth, "client")
	addr := serve(t, dir, nil)

	_, err := announceClient(t, dir).Disks(context.TODO(), addr)

	if err == nil {
		t.Error("a client certificate from an unrelated CA was accepted")
	}
}

func TestNewClientRejectsUnusableMaterial(t *testing.T) {
	dir := t.TempDir()
	ca, caKey := writeCA(t, dir)
	clientCert(t, dir, ca, caKey)
	caPEM := read(t, dir, CACert)
	certPEM := read(t, dir, ClientCert)
	keyPEM := read(t, dir, ClientKey)

	tests := []struct {
		name string
		ca   []byte
		cert []byte
		key  []byte
	}{
		{"a CA bundle with no certificates in it", []byte("not a certificate"), certPEM, keyPEM},
		{"a certificate that is not PEM", caPEM, []byte("not a certificate"), keyPEM},
		// The key of a different client: the pair does not go together, and
		// every handshake would fail at the point of proving it.
		{"a key that does not go with the certificate", caPEM, certPEM,
			func() []byte {
				other := t.TempDir()
				otherCA, otherKey := writeCA(t, other)
				clientCert(t, other, otherCA, otherKey)
				return read(t, other, ClientKey)
			}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.ca, tc.cert, tc.key)

			if err == nil {
				t.Error("NewClient accepted material it cannot use")
			}
		})
	}
}

// --- fixtures ---

// serveAnnounce writes a full set of TLS material into dir and starts a server
// announcing exports. The server certificate is issued for ServerName, which is
// what a real appliance's is.
func serveAnnounce(t *testing.T, dir string, exports []runner.Export) (addr string) {
	t.Helper()
	ca, caKey := writeCA(t, dir)
	writeLeaf(t, dir, "server", ca, caKey, x509.ExtKeyUsageServerAuth, ServerName)
	clientCert(t, dir, ca, caKey)
	return serve(t, dir, exports)
}

// serve starts a server on the material already in dir, on an ephemeral port.
func serve(t *testing.T, dir string, exports []runner.Export) (addr string) {
	t.Helper()
	server, err := New("127.0.0.1:0", dir, exports)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", server.http.TLSConfig)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})
	go func() {
		_ = server.http.Serve(listener)
	}()
	return listener.Addr().String()
}

// announceClient is a client built from the client half of what is in dir, the
// way the controller builds one from the TLS secret.
func announceClient(t *testing.T, dir string) *Client {
	t.Helper()
	client, err := NewClient(
		read(t, dir, CACert),
		read(t, dir, ClientCert),
		read(t, dir, ClientKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func read(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return data
}
