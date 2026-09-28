package provider

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const nutanixTestNamespace = "test"

func nutanixTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func nutanixProviderWithSecret(secretName string) *api.Provider {
	pt := api.Nutanix
	return &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: nutanixTestNamespace},
		Spec: api.ProviderSpec{
			Type: &pt,
			URL:  "https://prism-central.example.com:9440",
			Secret: core.ObjectReference{
				Name:      secretName,
				Namespace: nutanixTestNamespace,
			},
		},
	}
}

func newNutanixReconciler(t *testing.T, objs ...client.Object) Reconciler {
	t.Helper()
	scheme := nutanixTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return Reconciler{Reconciler: base.Reconciler{Client: cl}}
}

func TestValidateSecret_Nutanix_MissingCredentials(t *testing.T) {
	secret := &core.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "nutanix-secret", Namespace: nutanixTestNamespace},
		Data:       map[string][]byte{},
	}
	p := nutanixProviderWithSecret("nutanix-secret")
	r := newNutanixReconciler(t, secret)

	_, err := r.validateSecret(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Status.HasCondition(SecretNotValid) {
		t.Fatal("expected SecretNotValid condition when user/password are missing")
	}
	if p.Status.Phase != ValidationFailed {
		t.Errorf("expected phase %q, got %q", ValidationFailed, p.Status.Phase)
	}
}

func TestValidateSecret_Nutanix_MissingCACert(t *testing.T) {
	secret := &core.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "nutanix-secret", Namespace: nutanixTestNamespace},
		Data: map[string][]byte{
			"user":     []byte("admin"),
			"password": []byte("pass"),
		},
	}
	p := nutanixProviderWithSecret("nutanix-secret")
	r := newNutanixReconciler(t, secret)

	_, err := r.validateSecret(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Status.HasCondition(SecretNotValid) {
		t.Fatal("expected SecretNotValid condition when ca.crt is missing and insecureSkipVerify is not set")
	}
}

func TestValidateSecret_Nutanix_InsecureSkipVerify(t *testing.T) {
	secret := &core.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "nutanix-secret", Namespace: nutanixTestNamespace},
		Data: map[string][]byte{
			"user":               []byte("admin"),
			"password":           []byte("pass"),
			"insecureSkipVerify": []byte("true"),
		},
	}
	p := nutanixProviderWithSecret("nutanix-secret")
	r := newNutanixReconciler(t, secret)

	_, err := r.validateSecret(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status.HasCondition(SecretNotValid) {
		t.Fatal("did not expect SecretNotValid condition when credentials are present and TLS verification is explicitly skipped")
	}
	if !p.Status.HasCondition(ConnectionInsecure) {
		t.Fatal("expected ConnectionInsecure warning condition when insecureSkipVerify is true")
	}
}

// TestValidateSecret_Nutanix_ValidWithCACert exercises the full success path,
// including the active TLS handshake performed by validateTLSConnection, by
// standing up a real TLS server and trusting its certificate via ca.crt.
func TestValidateSecret_Nutanix_ValidWithCACert(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: server.Certificate().Raw,
	})

	secret := &core.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "nutanix-secret", Namespace: nutanixTestNamespace},
		Data: map[string][]byte{
			"user":     []byte("admin"),
			"password": []byte("pass"),
			"ca.crt":   caCertPEM,
		},
	}
	pt := api.Nutanix
	p := &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: nutanixTestNamespace},
		Spec: api.ProviderSpec{
			Type: &pt,
			URL:  server.URL,
			Secret: core.ObjectReference{
				Name:      "nutanix-secret",
				Namespace: nutanixTestNamespace,
			},
		},
	}
	r := newNutanixReconciler(t, secret)

	_, err := r.validateSecret(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status.HasCondition(SecretNotValid) {
		t.Fatal("did not expect SecretNotValid condition when credentials and ca.crt are present")
	}
	if p.Status.HasCondition(ConnectionTestFailed) {
		t.Fatal("did not expect ConnectionTestFailed condition when the TLS connection succeeds")
	}
	if p.Status.Phase == ConnectionFailed {
		t.Errorf("did not expect phase %q when the TLS connection succeeds", ConnectionFailed)
	}
}
