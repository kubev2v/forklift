package mutators

import (
	"encoding/json"
	"os"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"
)

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
