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

// Helper function to create an EC2 secret
func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName, accessKeyID, secretAccessKey, url, cacert, region string, insecureSkipTLS bool, targetAccessKeyID, targetSecretAccessKey string) (*corev1.Secret, error) {
	// Get the Kubernetes client using configFlags
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Create secret data without base64 encoding (the API handles this automatically)
	// URL is always included (either custom or default AWS endpoint)
	secretData := map[string][]byte{
		"accessKeyId":     []byte(accessKeyID),
		"secretAccessKey": []byte(secretAccessKey),
		"url":             []byte(url),
		"region":          []byte(region),
	}

	// Add optional fields
	if insecureSkipTLS {
		secretData["insecureSkipVerify"] = []byte("true")
	}
	if cacert != "" {
		secretData["cacert"] = []byte(cacert)
	}

	// Add cross-account migration credentials (optional)
	if targetAccessKeyID != "" {
		secretData["targetAccessKeyId"] = []byte(targetAccessKeyID)
	}
	if targetSecretAccessKey != "" {
		secretData["targetSecretAccessKey"] = []byte(targetSecretAccessKey)
	}

	// Generate a name prefix for the secret
	secretName := fmt.Sprintf("%s-ec2-", providerName)

	// Create the secret object directly as a typed Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: secretName,
			Namespace:    namespace,
			Labels: map[string]string{
				"createdForProviderType": "ec2",
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
