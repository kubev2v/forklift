package ec2

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// buildSecret returns an EC2 provider Secret without submitting it to the API.
func buildSecret(namespace, providerName, accessKeyID, secretAccessKey, url, cacert, region string, insecureSkipTLS bool, targetAccessKeyID, targetSecretAccessKey string) *corev1.Secret {
	secretData := map[string][]byte{
		"accessKeyId":     []byte(accessKeyID),
		"secretAccessKey": []byte(secretAccessKey),
		"url":             []byte(url),
		"region":          []byte(region),
	}

	if insecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}
	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	if targetAccessKeyID != "" {
		secretData["targetAccessKeyId"] = []byte(targetAccessKeyID)
	}
	if targetSecretAccessKey != "" {
		secretData["targetSecretAccessKey"] = []byte(targetSecretAccessKey)
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ec2-credentials", providerName),
			Namespace: namespace,
			Labels: map[string]string{
				"createdForProviderType": "ec2",
				"createdForResourceType": "providers",
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}
}

// createSecret creates an EC2 secret reusing the same object shape as buildSecret.
// It swaps the deterministic Name for a GenerateName so the API server assigns a unique suffix.
func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName, accessKeyID, secretAccessKey, url, cacert, region string, insecureSkipTLS bool, targetAccessKeyID, targetSecretAccessKey string) (*corev1.Secret, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secret := buildSecret(namespace, providerName, accessKeyID, secretAccessKey, url, cacert, region, insecureSkipTLS, targetAccessKeyID, targetSecretAccessKey)
	secret.Name = ""
	secret.GenerateName = fmt.Sprintf("%s-ec2-", providerName)

	return k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// setSecretOwnership sets the provider as the owner of the secret
func setSecretOwnership(configFlags *genericclioptions.ConfigFlags, provider *forkliftv1beta1.Provider, secret *corev1.Secret) error {
	// Get the Kubernetes client using configFlags
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Get the current secret to safely append owner reference
	currentSecret, err := k8sClient.CoreV1().Secrets(secret.Namespace).Get(
		context.Background(),
		secret.Name,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get secret for ownership update: %v", err)
	}

	// Create the owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion: provider.APIVersion,
		Kind:       provider.Kind,
		Name:       provider.Name,
		UID:        provider.UID,
	}

	// Check if this provider is already an owner to avoid duplicates
	for _, existingOwner := range currentSecret.OwnerReferences {
		if existingOwner.UID == provider.UID {
			return nil // Already an owner, nothing to do
		}
	}

	// Append the new owner reference to existing ones
	currentSecret.OwnerReferences = append(currentSecret.OwnerReferences, ownerRef)

	// Update the secret with the new owner reference
	_, err = k8sClient.CoreV1().Secrets(secret.Namespace).Update(
		context.Background(),
		currentSecret,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update secret with owner reference: %v", err)
	}

	return nil
}
