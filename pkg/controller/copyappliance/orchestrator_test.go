package copyappliance

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
)

// testLoadedImage is what LoadImage would have recorded by the time Configure
// runs, and so what the unit is rendered around.
const testLoadedImage = "localhost/forklift-copy-appliance:abc123def456"

func TestRenderUnit(t *testing.T) {
	t.Run("the unit describes this appliance", func(t *testing.T) {
		ac, _, orch := orchestratorLogin(t)
		ac.Appliance.Status.ExporterImage = testLoadedImage

		unit, err := orch.renderUnit()
		if err != nil {
			t.Fatalf("renderUnit: %v", err)
		}

		// The image is the only part that differs per appliance, and getting it
		// wrong means a supervisor that starts and then cannot run anything.
		for _, want := range []string{
			"-image=" + testLoadedImage,
			"-certs-dir=" + applianceCertsDir,
			"-listen=:" + applianceAnnouncePort,
			orchestratorBinary,
			// Without this the supervisor does not come back after a reboot,
			// which is the reason for using systemd at all.
			"WantedBy=multi-user.target",
			"Restart=always",
			"KillMode=process",
		} {
			if !strings.Contains(unit, want) {
				t.Errorf("the unit does not carry %q:\n%s", want, unit)
			}
		}
		if strings.Contains(unit, " -tls") {
			t.Errorf("plain unit should not enable -tls:\n%s", unit)
		}
	})

	// The image reference is the one thing the unit cannot be written without,
	// so rendering has to refuse rather than produce "-image=".
	t.Run("an appliance with no loaded image has no unit", func(t *testing.T) {
		_, _, orch := orchestratorLogin(t)

		_, err := orch.renderUnit()

		if err == nil {
			t.Error("rendered a unit for an appliance with no image")
		}
	})
}

func TestOrchestratorRestart(t *testing.T) {
	_, server, orch := orchestratorLogin(t)
	err := orch.Restart()
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	ran := server.Ran()
	for _, want := range []string{
		"systemctl reset-failed " + orchestratorUnit,
		"systemctl restart " + orchestratorUnit,
	} {
		if !slices.Contains(ran, want) {
			t.Errorf("%q was not run; ran %v", want, ran)
		}
	}
}

// --- fixtures ---

// orchestratorLogin is the supervisor on an appliance that refuses the named
// commands.
func orchestratorLogin(t *testing.T, failing ...string) (*ApplianceContext, *sshServer, *Orchestrator) {
	t.Helper()
	private, public := testKeyPair(t)
	server := startSSHServer(t, public, failing...)
	ac := sshContext(t, private, server.addr)
	orch := &Orchestrator{context: ac, ssh: loginAt(t, private, server.addr)}
	t.Cleanup(func() { _ = orch.Close() })
	return ac, server, orch
}

// tlsMaterial is one CA and the two halves signed by it.
type tlsMaterial struct {
	// data is the TLS secret as the operator creates it.
	data map[string][]byte
	// server is the keypair the appliance's announce endpoint serves with, and
	// pool the CA to verify it against.
	server tls.Certificate
	pool   *x509.CertPool
}

// applianceTLS is the material every test appliance is deployed with. Generated
// once for the whole binary, because the install probe hashes the certificates:
// two appliances holding different ones would disagree about what "already
// installed" means, and the point of the fixture is that they do not.
var applianceTLS = sync.OnceValue(newTLSMaterial)

func newTLSMaterial() *tlsMaterial {
	caKey, caCert, caPEM := issue(nil, nil, "Test CA", true, "")
	_, _, serverCertPEM, serverKeyPEM := leaf(caCert, caKey, announce.ServerName, x509.ExtKeyUsageServerAuth)
	_, _, clientCertPEM, clientKeyPEM := leaf(caCert, caKey, "client", x509.ExtKeyUsageClientAuth)

	server, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		panic(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return &tlsMaterial{
		data: map[string][]byte{
			announce.CACert:     caPEM,
			announce.ServerCert: serverCertPEM,
			announce.ServerKey:  serverKeyPEM,
			announce.ClientCert: clientCertPEM,
			announce.ClientKey:  clientKeyPEM,
		},
		server: server,
		pool:   pool,
	}
}

func leaf(ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string, eku x509.ExtKeyUsage) (*ecdsa.PrivateKey, *x509.Certificate, []byte, []byte) {
	key, cert, certPEM := issue(ca, caKey, name, false, "", eku)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return key, cert, certPEM, keyPEM
}

// issue signs a certificate, self-signing it when there is no parent. The name
// goes in both the subject and a DNS SAN, which is what a client verifies.
func issue(parent *x509.Certificate, parentKey *ecdsa.PrivateKey, name string, ca bool, _ string, eku ...x509.ExtKeyUsage) (*ecdsa.PrivateKey, *x509.Certificate, []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           eku,
		BasicConstraintsValid: true,
	}
	if ca {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.DNSNames = []string{name}
	}

	signer, signerKey := template, key
	if parent != nil {
		signer, signerKey = parent, parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, template, signer, &key.PublicKey, signerKey)
	if err != nil {
		panic(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}
	return key, cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
