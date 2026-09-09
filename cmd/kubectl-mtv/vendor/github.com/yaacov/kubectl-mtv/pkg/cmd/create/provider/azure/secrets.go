package azure

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

func buildSecret(namespace, providerName, tenantID, subscriptionID, clientID, clientSecret, resourceGroup string) *corev1.Secret {
	secretData := map[string][]byte{
		"tenantId":       []byte(tenantID),
		"subscriptionId": []byte(subscriptionID),
		"clientId":       []byte(clientID),
		"clientSecret":   []byte(clientSecret),
		"resourceGroup":  []byte(resourceGroup),
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-azure-", providerName),
			Namespace:    namespace,
			Labels: map[string]string{
				"createdForProviderType": string(flags.AzureProviderType),
				"createdForResourceType": "providers",
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}
}

func createSecret(configFlags *genericclioptions.ConfigFlags, namespace, providerName, tenantID, subscriptionID, clientID, clientSecret, resourceGroup string) (*corev1.Secret, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	secret := buildSecret(namespace, providerName, tenantID, subscriptionID, clientID, clientSecret, resourceGroup)

	return k8sClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

func setSecretOwnership(configFlags *genericclioptions.ConfigFlags, provider *forkliftv1beta1.Provider, secret *corev1.Secret) error {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	currentSecret, err := k8sClient.CoreV1().Secrets(secret.Namespace).Get(
		context.Background(),
		secret.Name,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get secret for ownership update: %v", err)
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: provider.APIVersion,
		Kind:       provider.Kind,
		Name:       provider.Name,
		UID:        provider.UID,
	}

	for _, existingOwner := range currentSecret.OwnerReferences {
		if existingOwner.UID == provider.UID {
			return nil
		}
	}

	currentSecret.OwnerReferences = append(currentSecret.OwnerReferences, ownerRef)

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
