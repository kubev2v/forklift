package azure

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
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

func validateProviderOptions(options providerutil.ProviderOptions) error {
	if options.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if options.Namespace == "" {
		return fmt.Errorf("provider namespace is required")
	}
	if options.Secret != "" && (options.AzureTenantID != "" || options.AzureSubscriptionID != "" || options.AzureClientID != "" || options.AzureClientSecret != "" || options.AzureResourceGroup != "") {
		return fmt.Errorf("if a secret is provided, Azure credential flags should not be specified")
	}
	if options.Secret == "" && (options.AzureTenantID == "" || options.AzureSubscriptionID == "" || options.AzureClientID == "" || options.AzureClientSecret == "" || options.AzureResourceGroup == "") {
		return fmt.Errorf("if no secret is provided, all Azure credentials must be specified (--azure-tenant-id, --azure-subscription-id, --azure-client-id, --azure-client-secret, --azure-resource-group)")
	}

	return nil
}

func cleanupCreatedResources(configFlags *genericclioptions.ConfigFlags, namespace string, secret *corev1.Secret) {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return
	}

	if secret != nil {
		_ = dynamicClient.Resource(client.SecretsGVR).Namespace(namespace).Delete(
			context.Background(),
			secret.Name,
			metav1.DeleteOptions{},
		)
	}
}

func createTypedProvider(configFlags *genericclioptions.ConfigFlags, namespace string, provider *forkliftv1beta1.Provider) (*forkliftv1beta1.Provider, error) {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	providerMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to convert provider to unstructured format: %v", err)
	}

	providerUnstructured := &unstructured.Unstructured{Object: providerMap}

	createdUnstructProvider, err := dynamicClient.Resource(client.ProvidersGVR).Namespace(namespace).Create(
		context.Background(),
		providerUnstructured,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, err
	}

	createdProvider := &forkliftv1beta1.Provider{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(createdUnstructProvider.Object, createdProvider); err != nil {
		return nil, fmt.Errorf("failed to convert provider from unstructured: %v", err)
	}

	return createdProvider, nil
}

// CreateProvider implements provider creation for Azure
func CreateProvider(configFlags *genericclioptions.ConfigFlags, options providerutil.ProviderOptions) (*forkliftv1beta1.Provider, *corev1.Secret, error) {
	if err := validateProviderOptions(options); err != nil {
		return nil, nil, err
	}

	provider := &forkliftv1beta1.Provider{}
	provider.SetName(options.Name)
	provider.SetNamespace(options.Namespace)
	provider.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()
	provider.Kind = "Provider"

	providerTypeValue := forkliftv1beta1.ProviderType(flags.AzureProviderType)
	provider.Spec.Type = &providerTypeValue

	providerURL := options.URL
	if providerURL == "" {
		providerURL = "https://management.azure.com"
	}
	provider.Spec.URL = providerURL

	if provider.Spec.Settings == nil {
		provider.Spec.Settings = map[string]string{}
	}

	if options.AzureTargetRegion != "" {
		provider.Spec.Settings["targetRegion"] = options.AzureTargetRegion
	}
	if options.AzureSnapshotSku != "" {
		provider.Spec.Settings["snapshotSku"] = options.AzureSnapshotSku
	}
	if options.AzureSnapshotResourceGroup != "" {
		provider.Spec.Settings["snapshotResourceGroup"] = options.AzureSnapshotResourceGroup
	}

	var createdSecret *corev1.Secret
	var err error

	if options.DryRun {
		if options.Secret == "" {
			createdSecret = buildSecret(options.Namespace, options.Name,
				options.AzureTenantID, options.AzureSubscriptionID,
				options.AzureClientID, options.AzureClientSecret,
				options.AzureResourceGroup)
			provider.Spec.Secret = corev1.ObjectReference{
				Name:      createdSecret.Name,
				Namespace: createdSecret.Namespace,
			}
		} else {
			provider.Spec.Secret = corev1.ObjectReference{
				Name:      options.Secret,
				Namespace: options.Namespace,
			}
		}
		return provider, createdSecret, nil
	}

	if options.Secret == "" {
		createdSecret, err = createSecret(configFlags, options.Namespace, options.Name,
			options.AzureTenantID, options.AzureSubscriptionID,
			options.AzureClientID, options.AzureClientSecret,
			options.AzureResourceGroup)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create Azure secret: %v", err)
		}

		provider.Spec.Secret = corev1.ObjectReference{
			Name:      createdSecret.Name,
			Namespace: createdSecret.Namespace,
		}
	} else {
		provider.Spec.Secret = corev1.ObjectReference{
			Name:      options.Secret,
			Namespace: options.Namespace,
		}
	}

	createdProvider, err := createTypedProvider(configFlags, options.Namespace, provider)
	if err != nil {
		cleanupCreatedResources(configFlags, options.Namespace, createdSecret)
		return nil, nil, fmt.Errorf("failed to create Azure provider: %v", err)
	}

	if createdSecret != nil {
		if err := setSecretOwnership(configFlags, createdProvider, createdSecret); err != nil {
			return nil, createdSecret, fmt.Errorf("provider created but %v", err)
		}
	}

	return createdProvider, createdSecret, nil
}
