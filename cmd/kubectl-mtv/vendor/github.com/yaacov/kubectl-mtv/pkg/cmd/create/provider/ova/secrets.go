package ova

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Helper function to create an OVA secret
func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName, url string) (*corev1.Secret, error) {
	// Get the Kubernetes client using configFlags
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Create secret data without base64 encoding (the API handles this automatically)
	secretData := map[string][]byte{
		"url": []byte(url),
	}

	// Generate a name prefix for the secret
	secretName := fmt.Sprintf("%s-ova-", providerName)

	// Create the secret object directly as a typed Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: secretName,
			Namespace:    namespace,
			Labels: map[string]string{
				"createdForProviderType": "ova",
				"createdForResourceType": "providers",
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}

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
