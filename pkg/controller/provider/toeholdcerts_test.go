package provider

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"

	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestToeholdTLSHandshakes is the only assertion that matters about the issued
// material: the appliance's announce endpoint and the controller's client must
// agree. Checking the certificate fields one by one would pass on material that
// still fails to connect, because what a Go verifier looks at is the server's
// DNS SAN and both leaves' extended key usages rather than anything in the
// subject.
func TestToeholdTLSHandshakes(t *testing.T) {
	data, err := toeholdTLS()
	if err != nil {
		t.Fatalf("toeholdTLS: %v", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data[announce.CACert]) {
		t.Fatalf("%s contained no certificates", announce.CACert)
	}
	serverCert, err := tls.X509KeyPair(data[announce.ServerCert], data[announce.ServerKey])
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	clientCert, err := tls.X509KeyPair(data[announce.ClientCert], data[announce.ClientKey])
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}

	// The appliance serves like announce.Server does, and the controller dials
	// like announce.NewClient does: pinned to the logical name, because the
	// appliance's address is not known when the certificate is issued.
	serverSide, clientSide := net.Pipe()
	// The pipe is torn down rather than the TLS connections: closing those
	// sends a close_notify that a pipe with nothing reading it cannot take, and
	// crypto/tls waits five seconds on it before giving up.
	defer func() {
		_ = serverSide.Close()
		_ = clientSide.Close()
	}()
	server := tls.Server(serverSide, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	})
	clientConn := tls.Client(clientSide, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientCert},
		ServerName:   announce.ServerName,
	})

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Handshake()
	}()
	err = clientConn.Handshake()
	if err != nil {
		t.Errorf("client handshake: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Errorf("server handshake: %v", err)
	}
}

// A secret predating the merge of the appliance's two secrets holds only the
// SSH key, and the key is the half that cannot be regenerated: its public half
// is already built into the toehold template.
func TestEnsureToeholdTLS(t *testing.T) {
	complete, err := toeholdTLS()
	if err != nil {
		t.Fatalf("toeholdTLS: %v", err)
	}
	complete["private-key"] = []byte("key")

	tests := []struct {
		name    string
		data    map[string][]byte
		wantNew bool
	}{
		{
			name:    "a secret holding only the SSH key gains the certificates",
			data:    map[string][]byte{"private-key": []byte("key")},
			wantNew: true,
		},
		{
			name:    "a complete secret is left alone",
			data:    complete,
			wantNew: false,
		},
		{
			name: "a secret missing one certificate has all of them replaced",
			data: map[string][]byte{
				"private-key":       []byte("key"),
				announce.CACert:     complete[announce.CACert],
				announce.ServerCert: complete[announce.ServerCert],
				announce.ServerKey:  complete[announce.ServerKey],
				announce.ClientCert: complete[announce.ClientCert],
			},
			wantNew: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &core.Secret{
				ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "toehold-ssh-keys-vcenter-private"},
				Data:       tt.data,
			}
			reconciler := testCertsReconciler(t, secret)

			err := reconciler.ensureToeholdTLS(secret)
			if err != nil {
				t.Fatalf("ensureToeholdTLS: %v", err)
			}

			// The SSH key is the whole reason the secret is filled in rather
			// than recreated.
			if got := string(secret.Data["private-key"]); got != "key" {
				t.Errorf("private-key = %q, want it untouched", got)
			}
			for _, key := range announce.SecretTLSKeys {
				if len(secret.Data[key]) == 0 {
					t.Errorf("%s is empty, want it filled in", key)
				}
			}
			// Replaced together or not at all: a leaf and a CA from different
			// runs do not chain.
			replaced := string(secret.Data[announce.CACert]) != string(complete[announce.CACert])
			if replaced != tt.wantNew {
				t.Errorf("regenerated = %v, want %v", replaced, tt.wantNew)
			}
		})
	}
}

func testCertsReconciler(t *testing.T, objs ...client.Object) Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme core: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return Reconciler{Reconciler: base.Reconciler{Client: cl, Log: logging.WithName("test")}}
}
