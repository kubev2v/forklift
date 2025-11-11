package vsphere

import (
	"context"
	"fmt"
	"strconv"
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

// validateProviderOptions validates the options for creating a vSphere provider
func validateProviderOptions(options providerutil.ProviderOptions) error {
	if options.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if options.Namespace == "" {
		return fmt.Errorf("provider namespace is required")
	}
	if options.URL == "" {
		return fmt.Errorf("provider URL is required")
	}
	if options.Username == "" {
		return fmt.Errorf("provider username is required")
	}
	if options.Password == "" {
		return fmt.Errorf("provider password is required")
	}
	if options.CACert == "" && !options.InsecureSkipTLS {
		return fmt.Errorf("either CA certificate or insecure skip TLS must be provided")
	}
	if options.Secret != "" && (options.Username != "" || options.Password != "") {
		return fmt.Errorf("if a secret is provided, username and password should not be specified")
	}
	if options.Secret == "" && (options.Username == "" || options.Password == "") {
		return fmt.Errorf("if no secret is provided, username and password must be specified")
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

// CreateProvider implements the ProviderCreator interface for VSphere
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

	// Set provider type and URL
	providerTypeValue := forkliftv1beta1.ProviderType("vsphere")
	provider.Spec.Type = &providerTypeValue
	provider.Spec.URL = options.URL

	// Initialize settings map if any settings are provided
	if options.VddkInitImage != "" || options.SdkEndpoint != "" || options.UseVddkAioOptimization ||
		options.VddkBufSizeIn64K > 0 || options.VddkBufCount > 0 {
		provider.Spec.Settings = map[string]string{}
	}

	// Set VDDK init image if provided
	if options.VddkInitImage != "" {
		provider.Spec.Settings["vddkInitImage"] = options.VddkInitImage
	}

	// Set SDK endpoint if provided
	if options.SdkEndpoint != "" {
		provider.Spec.Settings["sdkEndpoint"] = options.SdkEndpoint
	}

	// Set VDDK AIO optimization if enabled
	if options.UseVddkAioOptimization {
		provider.Spec.Settings["useVddkAioOptimization"] = "true"
	}

	// Set VDDK configuration if buffer settings are provided
	if options.VddkBufSizeIn64K > 0 || options.VddkBufCount > 0 {
		var vddkConfig strings.Builder

		// Start with YAML literal block scalar format
		vddkConfig.WriteString("|")

		if options.VddkBufSizeIn64K > 0 {
			vddkConfig.WriteString("\nVixDiskLib.nfcAio.Session.BufSizeIn64K=")
			vddkConfig.WriteString(strconv.Itoa(options.VddkBufSizeIn64K))
		}

		if options.VddkBufCount > 0 {
			vddkConfig.WriteString("\nVixDiskLib.nfcAio.Session.BufCount=")
			vddkConfig.WriteString(strconv.Itoa(options.VddkBufCount))
		}

		provider.Spec.Settings["vddkConfig"] = vddkConfig.String()
	}

	// Create and set the Secret
	var createdSecret *corev1.Secret
	var err error

	if options.Secret == "" {
		createdSecret, err = createSecret(configFlags, options.Namespace, options.Name,
			options.Username, options.Password, options.URL, options.CACert, options.InsecureSkipTLS)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create vSphere secret: %v", err)
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

		return nil, nil, fmt.Errorf("failed to create vSphere provider: %v", err)
	}

	// Set the secret ownership to the provider
	if createdSecret != nil {
		if err := setSecretOwnership(configFlags, createdProvider, createdSecret); err != nil {
			return nil, createdSecret, fmt.Errorf("provider created but %v", err)
		}
	}

	return createdProvider, createdSecret, nil
}
