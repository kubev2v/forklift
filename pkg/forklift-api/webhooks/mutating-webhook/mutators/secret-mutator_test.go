package mutators

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"
)

// selfSignedCertPEM returns a valid self-signed certificate in PEM format.
func selfSignedCertPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-engine"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestCertAppending(t *testing.T) {
	g := NewGomegaWithT(t)

	providedCa, err := os.ReadFile("completeCerts.pem")
	g.Expect(err).ToNot(HaveOccurred())

	newCa, err := os.ReadFile("engineCert.pem")
	g.Expect(err).ToNot(HaveOccurred())

	//Test the case where two certificates are identical but have a different line count due to new lines.
	g.Expect(contains(providedCa, newCa)).To(BeTrue())

	//Test the case where the original certificate does not have a new line at the end.
	g.Expect(appendCerts(newCa, newCa)).To(ContainSubstring("-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----"))

	//Test the case where the original certificate has a new line at the end and verify a redundant new line was not added.
	newCa = append(newCa, 0x0a)
	g.Expect(appendCerts(newCa, newCa)).To(ContainSubstring("-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----"))

	//Test the case when the certificate is changed by one byte to verify that "contains" returns false.
	newCa = append(newCa, 0x01)
	g.Expect(contains(providedCa, newCa)).To(BeFalse())
}

func TestMutateHostSecretInsecureSkipVerify(t *testing.T) {
	g := NewGomegaWithT(t)

	// Create a provider with insecureSkipVerify setting
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider",
			Namespace: "test-namespace",
		},
		Spec: api.ProviderSpec{
			Secret: core.ObjectReference{
				Name:      "provider-secret",
				Namespace: "test-namespace",
			},
		},
	}

	// Create provider secret with insecureSkipVerify
	providerSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"insecureSkipVerify": []byte("true"),
			"user":               []byte("admin"),
			"password":           []byte("password"),
		},
	}

	// Create host secret without insecureSkipVerify
	hostSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-secret",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"createdForResourceType": "hosts",
				"createdForResource":     "test-host",
			},
		},
		Data: map[string][]byte{
			"provider": []byte("test-provider"),
			"user":     []byte("host-user"),
			"password": []byte("host-password"),
			// Note: no insecureSkipVerify key
		},
	}

	// Create fake client with the objects
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = api.SchemeBuilder.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(provider, providerSecret).
		Build()

	// Create admission request
	hostSecretBytes, err := json.Marshal(hostSecret)
	g.Expect(err).ToNot(HaveOccurred())

	admissionRequest := &admissionv1.AdmissionRequest{
		Object: runtime.RawExtension{
			Raw: hostSecretBytes,
		},
	}

	admissionReview := &admissionv1.AdmissionReview{
		Request: admissionRequest,
	}

	// Create mutator and test
	mutator := &SecretMutator{
		Client: fakeClient,
	}

	response := mutator.Mutate(admissionReview)

	// Verify the response
	g.Expect(response.Allowed).To(BeTrue())
	g.Expect(response.Patch).ToNot(BeNil())
	g.Expect(response.PatchType).ToNot(BeNil())
	g.Expect(*response.PatchType).To(Equal(admissionv1.PatchTypeJSONPatch))
}

func TestMutateHostSecretWithInsecureSkipVerify(t *testing.T) {
	g := NewGomegaWithT(t)

	// Create host secret with insecureSkipVerify already present
	hostSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-secret",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"createdForResourceType": "hosts",
				"createdForResource":     "test-host",
			},
		},
		Data: map[string][]byte{
			"provider":           []byte("test-provider"),
			"user":               []byte("host-user"),
			"password":           []byte("host-password"),
			"insecureSkipVerify": []byte("false"), // Already present
		},
	}

	// Create fake client
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = api.SchemeBuilder.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create admission request
	hostSecretBytes, err := json.Marshal(hostSecret)
	g.Expect(err).ToNot(HaveOccurred())

	admissionRequest := &admissionv1.AdmissionRequest{
		Object: runtime.RawExtension{
			Raw: hostSecretBytes,
		},
	}

	admissionReview := &admissionv1.AdmissionReview{
		Request: admissionRequest,
	}

	// Create mutator and test
	mutator := &SecretMutator{
		Client: fakeClient,
	}

	response := mutator.Mutate(admissionReview)

	// Verify the response - should be allowed but no patch since insecureSkipVerify already exists
	g.Expect(response.Allowed).To(BeTrue())
	g.Expect(response.Patch).To(BeNil()) // No patch needed
}

// A no-op rewrite of a provider secret (e.g. by the storage version migrator or
// an encryption key rotation) must be allowed without external calls (MTV-7041).
func TestMutateProviderSecretNoOpUpdate(t *testing.T) {
	g := NewGomegaWithT(t)

	cacert := selfSignedCertPEM(t)

	newSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider-secret",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"createdForResourceType": "providers",
				"createdForProviderType": "ovirt",
			},
		},
		Data: map[string][]byte{
			"url":  []byte("https://127.0.0.1:1/ovirt-engine/api"),
			"user": []byte("admin"),
			// Note: no insecureSkipVerify key - if the no-op check is broken the
			// mutator would default it and return a patch.
			"cacert": cacert,
		},
	}
	// The rewrite only differs in metadata, not in Data or Labels.
	oldSecret := newSecret.DeepCopy()
	oldSecret.ResourceVersion = "1"

	newBytes, err := json.Marshal(newSecret)
	g.Expect(err).ToNot(HaveOccurred())
	oldBytes, err := json.Marshal(oldSecret)
	g.Expect(err).ToNot(HaveOccurred())

	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    runtime.RawExtension{Raw: newBytes},
			OldObject: runtime.RawExtension{Raw: oldBytes},
		},
	}

	response := (&SecretMutator{}).Mutate(admissionReview)

	g.Expect(response.Allowed).To(BeTrue())
	g.Expect(response.Patch).To(BeNil())
}

// A changed provider secret without insecureSkipVerify must receive the
// default in its patch.
func TestMutateProviderSecretUpdateAddsInsecureSkipVerifyDefault(t *testing.T) {
	g := NewGomegaWithT(t)

	newSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider-secret",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"createdForResourceType": "providers",
				"createdForProviderType": "vsphere",
			},
		},
		Data: map[string][]byte{
			"url":  []byte("https://vcenter.example.com/sdk"),
			"user": []byte("admin"),
		},
	}
	oldSecret := newSecret.DeepCopy()
	oldSecret.Data["user"] = []byte("old-admin")

	newBytes, err := json.Marshal(newSecret)
	g.Expect(err).ToNot(HaveOccurred())
	oldBytes, err := json.Marshal(oldSecret)
	g.Expect(err).ToNot(HaveOccurred())

	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    runtime.RawExtension{Raw: newBytes},
			OldObject: runtime.RawExtension{Raw: oldBytes},
		},
	}

	response := (&SecretMutator{}).Mutate(admissionReview)

	g.Expect(response.Allowed).To(BeTrue())
	g.Expect(response.Patch).ToNot(BeNil())

	var ops []struct {
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	}
	g.Expect(json.Unmarshal(response.Patch, &ops)).To(Succeed())
	var dataValue map[string][]byte
	for _, op := range ops {
		if op.Path == "/data" {
			g.Expect(json.Unmarshal(op.Value, &dataValue)).To(Succeed())
		}
	}
	g.Expect(dataValue).To(HaveKeyWithValue("insecureSkipVerify", []byte("false")))
}

// A changed provider secret must not be denied when the oVirt engine is
// unreachable (MTV-7041).
func TestMutateProviderSecretUpdateWithUnreachableEngine(t *testing.T) {
	g := NewGomegaWithT(t)

	cacert := selfSignedCertPEM(t)

	newSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider-secret",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"createdForResourceType": "providers",
				"createdForProviderType": "ovirt",
			},
		},
		Data: map[string][]byte{
			"url":                []byte("https://127.0.0.1:1/ovirt-engine/api"),
			"insecureSkipVerify": []byte("false"),
			"cacert":             cacert,
		},
	}
	oldSecret := newSecret.DeepCopy()
	oldSecret.Data["url"] = []byte("https://old-engine.example.com/ovirt-engine/api")

	newBytes, err := json.Marshal(newSecret)
	g.Expect(err).ToNot(HaveOccurred())
	oldBytes, err := json.Marshal(oldSecret)
	g.Expect(err).ToNot(HaveOccurred())

	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    runtime.RawExtension{Raw: newBytes},
			OldObject: runtime.RawExtension{Raw: oldBytes},
		},
	}

	response := (&SecretMutator{}).Mutate(admissionReview)

	// The engine is unreachable: allowed without updating the CA.
	g.Expect(response.Allowed).To(BeTrue())
	g.Expect(response.Patch).To(BeNil())
}

func TestFetchEngineCACertificate(t *testing.T) {
	g := NewGomegaWithT(t)

	// A non-200 response must be an error, not a body appended to the secret.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := fetchEngineCACertificate(server.URL, x509.NewCertPool())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("404"))

	expectedCert := []byte("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n")
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(expectedCert)
	}))
	defer server.Close()

	cert, err := fetchEngineCACertificate(server.URL, x509.NewCertPool())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cert).To(Equal(expectedCert))
}

// The fetch must verify the engine against the provided CA pool: it is the
// mismatch between the engine certificate and the secret's CA that triggered
// MTV-7041.
func TestFetchEngineCACertificateTLS(t *testing.T) {
	g := NewGomegaWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("engine-ca"))
	}))
	defer server.Close()

	trusted := x509.NewCertPool()
	trusted.AddCert(server.Certificate())
	cert, err := fetchEngineCACertificate(server.URL, trusted)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cert).To(Equal([]byte("engine-ca")))

	unrelated := x509.NewCertPool()
	g.Expect(unrelated.AppendCertsFromPEM(selfSignedCertPEM(t))).To(BeTrue())
	_, err = fetchEngineCACertificate(server.URL, unrelated)
	g.Expect(err).To(HaveOccurred())
}

// A stalled engine response must be bounded by the client timeout.
func TestFetchEngineCACertificateStalledEngine(t *testing.T) {
	g := NewGomegaWithT(t)

	stalled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-stalled:
		}
	}))
	defer func() {
		close(stalled)
		server.Close()
	}()

	done := make(chan error, 1)
	go func() {
		_, err := fetchEngineCACertificate(server.URL, x509.NewCertPool())
		done <- err
	}()

	select {
	case err := <-done:
		g.Expect(err).To(HaveOccurred(), "expected the stalled engine response to fail")
	case <-time.After(15 * time.Second):
		t.Fatal("fetchEngineCACertificate did not return within 15s; the stalled engine response was not bounded")
	}
}
