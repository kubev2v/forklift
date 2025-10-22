//nolint:errcheck
package ocp

import (
	"os"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setupKubeConfig(t *testing.T) (cleanupFunc func()) {
	tempFile, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	kubeconfig := `
apiVersion: v1
clusters:
- cluster:
    server: http://localhost
  name: fake
contexts:
- context:
    cluster: fake
    user: ""
  name: fake
current-context: fake
kind: Config
preferences: {}
`

	os.WriteFile(tempFile.Name(), []byte(kubeconfig), 0644)
	os.Setenv("KUBECONFIG", tempFile.Name())

	return func() {
		os.Unsetenv("KUBECONFIG")
		os.Remove(tempFile.Name())
	}
}

func TestRestCfg(t *testing.T) {
	testCases := []struct {
		name             string
		provider         *api.Provider
		secretData       map[string][]byte
		expectedInsecure bool
		expectedCAData   []byte
	}{
		{
			name: "Insecure true, no cacert",
			provider: &api.Provider{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       api.ProviderSpec{URL: "https://test.com"},
			},
			secretData: map[string][]byte{
				api.Token:    []byte("token"),
				api.Insecure: []byte("true"),
			},
			expectedInsecure: true,
			expectedCAData:   nil,
		},
		{
			name: "Insecure false, cacert present",
			provider: &api.Provider{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Spec:       api.ProviderSpec{URL: "https://test.com"},
			},
			secretData: map[string][]byte{
				api.Token:    []byte("token"),
				api.Insecure: []byte("false"),
				"cacert":     []byte("certData"),
			},
			expectedInsecure: false,
			expectedCAData:   []byte("certData"),
		},
	}

	cleanup := setupKubeConfig(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			secret := &core.Secret{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
				Data:       tc.secretData,
			}

			config := RestCfg(tc.provider, secret)
			if config == nil {
				t.Fatalf("Expected non-nil config")
			}

			if config != nil && config.TLSClientConfig.Insecure != tc.expectedInsecure {
				t.Errorf("Expected TLSClientConfig.Insecure to be %v, got %v", tc.expectedInsecure, config.TLSClientConfig.Insecure)
			}

			if config != nil && string(config.TLSClientConfig.CAData) != string(tc.expectedCAData) {
				t.Errorf("Expected TLSClientConfig.CAData to be %s, got %s", string(tc.expectedCAData), string(config.TLSClientConfig.CAData))
			}
		})
	}

	cleanup()
}
