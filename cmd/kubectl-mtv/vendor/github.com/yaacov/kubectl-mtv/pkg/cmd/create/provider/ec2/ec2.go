package ec2

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/providerutil"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// getAWSEndpoint returns the appropriate AWS EC2 endpoint based on region
// Handles different AWS partitions (standard, China, GovCloud)
func getAWSEndpoint(region string) string {
	// China regions use .cn domain
	if strings.HasPrefix(region, "cn-") {
		return fmt.Sprintf("https://ec2.%s.amazonaws.com.cn", region)
	}
	// GovCloud regions
	if strings.HasPrefix(region, "us-gov-") {
		return fmt.Sprintf("https://ec2.%s.amazonaws.com", region)
	}
	// Standard AWS partition
	return fmt.Sprintf("https://ec2.%s.amazonaws.com", region)
}

// validateProviderOptions validates the options for creating an EC2 provider
func validateProviderOptions(options providerutil.ProviderOptions) error {
	if options.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if options.Namespace == "" {
		return fmt.Errorf("provider namespace is required")
	}
	// URL is optional for EC2 providers (AWS uses regional endpoints)
	if options.EC2Region == "" {
		return fmt.Errorf("EC2 region is required")
	}
	// For EC2, CA cert and insecure skip TLS are optional (AWS certificates are typically trusted)
	if options.Secret != "" && (options.Username != "" || options.Password != "") {
		return fmt.Errorf("if a secret is provided, username and password should not be specified")
	}
	if options.Secret == "" && (options.Username == "" || options.Password == "") {
		return fmt.Errorf("if no secret is provided, both access key ID (username) and secret access key (password) must be specified")
	}

	return nil
}

// cleanupCreatedResources deletes any resources created during the provider creation process
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

// CreateProvider implements the ProviderCreator interface for EC2
func CreateProvider(configFlags *genericclioptions.ConfigFlags, options providerutil.ProviderOptions) (*forkliftv1beta1.Provider, *corev1.Secret, error) {
	// Auto-fetch target credentials and target-az from cluster if requested
	if options.AutoTargetCredentials {
		if err := AutoPopulateTargetOptions(configFlags, &options.EC2TargetAccessKeyID, &options.EC2TargetSecretKey, &options.EC2TargetAZ, &options.EC2TargetRegion); err != nil {
			return nil, nil, err
		}
	}

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

	// Set provider type
	providerTypeValue := forkliftv1beta1.ProviderType("ec2")
	provider.Spec.Type = &providerTypeValue

	// Set URL - use provided URL or construct default AWS regional endpoint
	providerURL := options.URL
	if providerURL == "" {
		// Construct default AWS EC2 regional endpoint
		// Handle different AWS partitions (China regions use .cn domain)
		providerURL = getAWSEndpoint(options.EC2Region)
	}
	provider.Spec.URL = providerURL

	// Always set target-region: use provided value, or default to provider region
	targetRegion := options.EC2TargetRegion
	if targetRegion == "" {
		targetRegion = options.EC2Region
	}

	// Always set target-az: use provided value, or default to target-region + 'a'
	targetAZ := options.EC2TargetAZ
	if targetAZ == "" {
		targetAZ = targetRegion + "a"
	}

	// Initialize settings map and set EC2-specific settings
	if provider.Spec.Settings == nil {
		provider.Spec.Settings = map[string]string{}
	}
	provider.Spec.Settings["target-region"] = targetRegion
	provider.Spec.Settings["target-az"] = targetAZ

	// Create and set the Secret
	var createdSecret *corev1.Secret
	var err error

	if options.Secret == "" {
		// Pass the providerURL (which may be default or custom) to secret creation
		createdSecret, err = createSecret(configFlags, options.Namespace, options.Name,
			options.Username, options.Password, providerURL, options.CACert, options.EC2Region, options.InsecureSkipTLS,
			options.EC2TargetAccessKeyID, options.EC2TargetSecretKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create EC2 secret: %v", err)
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

	// Create the provider
	createdProvider, err := createTypedProvider(configFlags, options.Namespace, provider)
	if err != nil {
		// Clean up the created secret if provider creation fails
		cleanupCreatedResources(configFlags, options.Namespace, createdSecret)

		return nil, nil, fmt.Errorf("failed to create EC2 provider: %v", err)
	}

	// Set the secret ownership to the provider
	if createdSecret != nil {
		if err := setSecretOwnership(configFlags, createdProvider, createdSecret); err != nil {
			return nil, createdSecret, fmt.Errorf("provider created but %v", err)
		}
	}

	return createdProvider, createdSecret, nil
}
