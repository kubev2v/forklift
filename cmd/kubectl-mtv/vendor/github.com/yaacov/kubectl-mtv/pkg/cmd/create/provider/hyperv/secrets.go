package hyperv

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/providerutil"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// buildSecret returns a HyperV provider Secret without submitting it to the API.
func buildSecret(namespace, providerName string, options providerutil.ProviderOptions) (*corev1.Secret, error) {
	if options.Username == "" || options.Password == "" {
		return nil, fmt.Errorf("username and password are required for HyperV provider")
	}

	secretData := map[string][]byte{
		"username": []byte(options.Username),
		"password": []byte(options.Password),
	}
	if options.SMBUrl != "" {
		secretData["smbUrl"] = []byte(options.SMBUrl)
	}
	if options.SMBUser != "" {
		secretData["smbUser"] = []byte(options.SMBUser)
	}
	if options.SMBPassword != "" {
		secretData["smbPassword"] = []byte(options.SMBPassword)
	}
	if options.InsecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}
	if options.CACert != "" {
		secretData["cacert"] = []byte(options.CACert)
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-hyperv-credentials", providerName),
			Namespace: namespace,
			Labels: map[string]string{
				"createdForProviderType": "hyperv",
				"createdForResourceType": "providers",
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}, nil
}

// createSecret creates a HyperV secret reusing the same object shape as buildSecret.
// It swaps the deterministic Name for a GenerateName so the API server assigns a unique suffix.
func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName string, options providerutil.ProviderOptions) (*corev1.Secret, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secret, err := buildSecret(namespace, providerName, options)
	if err != nil {
		return nil, err
	}
	secret.Name = ""
	secret.GenerateName = fmt.Sprintf("%s-hyperv-", providerName)

	return k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// setSecretOwnership sets the provider as the owner of the secret
func setSecretOwnership(configFlags *genericclioptions.ConfigFlags, provider *forkliftv1beta1.Provider, secret *corev1.Secret) error {
	// Get the Kubernetes client using configFlags
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Create the owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion: provider.APIVersion,
		Kind:       provider.Kind,
		Name:       provider.Name,
		UID:        provider.UID,
	}

	// Patch secret to add the owner reference
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"ownerReferences": []metav1.OwnerReference{ownerRef},
		},
	}

	// Convert patch to JSON bytes
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %v", err)
	}

	// Apply the patch to the secret
	_, err = k8sClient.CoreV1().Secrets(secret.Namespace).Patch(
		context.Background(),
		secret.Name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch secret with owner reference: %v", err)
	}

	return nil
}
