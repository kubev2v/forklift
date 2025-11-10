package openshift

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/providerutil"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// validateProviderOptions validates the options for creating an OpenShift provider
func validateProviderOptions(options providerutil.ProviderOptions) error {
	if options.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if options.Namespace == "" {
		return fmt.Errorf("provider namespace is required")
	}
	// URL is only required when token is provided
	if options.Token != "" && options.URL == "" {
		return fmt.Errorf("provider URL is required when token is provided")
	}

	return nil
}

// cleanupCreatedResources deletes any resources created during the provider creation process
func cleanupCreatedResources(configFlags *genericclioptions.ConfigFlags, namespace string, secret *corev1.Secret) {
	if secret == nil {
		return
	}

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return
	}

	_ = dynamicClient.Resource(client.SecretsGVR).Namespace(namespace).Delete(
		context.Background(),
		secret.Name,
		metav1.DeleteOptions{},
	)
}

// createTypedProvider creates an unstructured provider and converts it to a typed Provider
func createTypedProvider(configFlags *genericclioptions.ConfigFlags, namespace string, provider *forkliftv1beta1.Provider) (*forkliftv1beta1.Provider, error) {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	// Convert the provider object to unstructured format
	providerMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to convert provider to unstructured format: %v", err)
	}

	// Create an *unstructured.Unstructured from the map
	providerUnstructured := &unstructured.Unstructured{Object: providerMap}

	createdUnstructProvider, err := dynamicClient.Resource(client.ProvidersGVR).Namespace(namespace).Create(
		context.Background(),
		providerUnstructured,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, err
	}

	// Convert unstructured provider to typed provider
	createdProvider := &forkliftv1beta1.Provider{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(createdUnstructProvider.Object, createdProvider); err != nil {
		return nil, fmt.Errorf("failed to convert provider from unstructured: %v", err)
	}

	return createdProvider, nil
}

// CreateProvider implements the ProviderCreator interface for OpenShift
func CreateProvider(configFlags *genericclioptions.ConfigFlags, options providerutil.ProviderOptions) (*forkliftv1beta1.Provider, *corev1.Secret, error) {
	// Validate required fields
	if err := validateProviderOptions(options); err != nil {
		return nil, nil, err
	}

	// Create basic provider structure
	provider := &forkliftv1beta1.Provider{}
	provider.SetName(options.Name)
	provider.SetNamespace(options.Namespace)
	provider.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()
	provider.Kind = "Provider"

	// Set provider type and URL (if provided)
	providerTypeValue := forkliftv1beta1.ProviderType("openshift")
	provider.Spec.Type = &providerTypeValue
	if options.URL != "" {
		provider.Spec.URL = options.URL
	}

	// Handle secret and token logic
	var createdSecret *corev1.Secret
	var err error

	if options.Token != "" {
		createdSecret, err = createSecret(configFlags, options.Namespace, options.Name,
			options.URL, options.Token, options.CACert, options.InsecureSkipTLS)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create vSphere secret: %v", err)
		}

		// If token is provided
		if options.Secret != "" {
			// Use the provided secret if specified alongside the token
			provider.Spec.Secret = corev1.ObjectReference{
				Name:      options.Secret,
				Namespace: options.Namespace,
			}
		} else {
			// Create a new secret if no existing secret is specified
			createdSecret, err = createSecret(configFlags, options.Namespace, options.Name, options.URL, options.Token, options.CACert, options.InsecureSkipTLS)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create OpenShift secret: %v", err)
			}

			provider.Spec.Secret = corev1.ObjectReference{
				Name:      createdSecret.Name,
				Namespace: createdSecret.Namespace,
			}
		}
	}

	// Create the provider
	createdProvider, err := createTypedProvider(configFlags, options.Namespace, provider)
	if err != nil {
		// Clean up the created secret if provider creation fails and we created it
		if createdSecret != nil {
			cleanupCreatedResources(configFlags, options.Namespace, createdSecret)
		}

		return nil, nil, fmt.Errorf("failed to create OpenShift provider: %v", err)
	}

	// Set the secret ownership to the provider if we created a secret
	if createdSecret != nil {
		if err := setSecretOwnership(configFlags, createdProvider, createdSecret); err != nil {
			return nil, createdSecret, fmt.Errorf("provider created but %v", err)
		}
	}

	return createdProvider, createdSecret, nil
}
