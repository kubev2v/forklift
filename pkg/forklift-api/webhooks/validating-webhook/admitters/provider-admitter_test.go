package admitters

import (
	"encoding/json"
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

func typedProvider(providerType api.ProviderType, settings map[string]string) api.Provider {
	return api.Provider{
		Spec: api.ProviderSpec{
			Type:     &providerType,
			Settings: settings,
		},
	}
}

// admitProvider runs the provider admitter on an admission review carrying
// the given provider, the way the webhook server does.
func admitProvider(t *testing.T, provider api.Provider, operation admissionv1.Operation) *admissionv1.AdmissionResponse {
	t.Helper()
	raw, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("marshal provider: %v", err)
	}
	review := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: operation,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
	return (&ProviderAdmitter{}).Admit(review)
}

// TestAdmitDatacenterSetting asserts the admission behavior of the
// datacenter setting: the combination with a direct ESXi SDK endpoint is
// rejected on create and update, and every other combination — including
// the setting on non-vSphere providers — is allowed.
func TestAdmitDatacenterSetting(t *testing.T) {
	cases := []struct {
		name        string
		provider    api.Provider
		operation   admissionv1.Operation
		allowed     bool
		wantMessage []string // substrings expected in the rejection message
	}{
		{
			name:        "rejects datacenter with esxi endpoint on create",
			provider:    typedProvider(api.VSphere, map[string]string{api.Datacenter: "DC0", api.SDK: api.ESXI}),
			operation:   admissionv1.Create,
			allowed:     false,
			wantMessage: []string{"datacenter", "ESXi"},
		},
		{
			name:        "rejects datacenter with esxi endpoint on update",
			provider:    typedProvider(api.VSphere, map[string]string{api.Datacenter: "DC0", api.SDK: api.ESXI}),
			operation:   admissionv1.Update,
			allowed:     false,
			wantMessage: []string{"datacenter", "ESXi"},
		},
		{
			name:      "accepts datacenter with vcenter endpoint",
			provider:  typedProvider(api.VSphere, map[string]string{api.Datacenter: "DC0", api.SDK: api.VCenter}),
			operation: admissionv1.Create,
			allowed:   true,
		},
		{
			name:      "accepts datacenter without sdk endpoint",
			provider:  typedProvider(api.VSphere, map[string]string{api.Datacenter: "DC0"}),
			operation: admissionv1.Create,
			allowed:   true,
		},
		{
			name:      "accepts empty datacenter with esxi endpoint",
			provider:  typedProvider(api.VSphere, map[string]string{api.Datacenter: "", api.SDK: api.ESXI}),
			operation: admissionv1.Create,
			allowed:   true,
		},
		{
			name:      "ignores datacenter on non-vSphere providers",
			provider:  typedProvider(api.OVirt, map[string]string{api.Datacenter: "DC0", api.SDK: api.ESXI}),
			operation: admissionv1.Create,
			allowed:   true,
		},
		{
			name:        "still rejects an invalid sdk endpoint",
			provider:    typedProvider(api.VSphere, map[string]string{api.Datacenter: "DC0", api.SDK: "bogus"}),
			operation:   admissionv1.Create,
			allowed:     false,
			wantMessage: []string{"invalid SDK endpoint type"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			response := admitProvider(t, tt.provider, tt.operation)
			if response.Allowed != tt.allowed {
				t.Fatalf("admission allowed = %v, want %v", response.Allowed, tt.allowed)
			}
			if tt.allowed {
				return
			}
			if response.Result == nil {
				t.Fatal("rejected admission response has no result")
			}
			for _, want := range tt.wantMessage {
				if !strings.Contains(response.Result.Message, want) {
					t.Errorf("rejection message %q does not contain %q", response.Result.Message, want)
				}
			}
		})
	}
}
