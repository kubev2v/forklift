package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Group is the API group for MTV CRDs
const Group = "forklift.konveyor.io"

// Version is the API version for MTV CRDs
const Version = "v1beta1"

// Resource GVRs for MTV CRDs
var (
	ProvidersGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "providers",
	}

	NetworkMapGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "networkmaps",
	}

	StorageMapGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "storagemaps",
	}

	PlansGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "plans",
	}

	MigrationsGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "migrations",
	}

	HostsGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "hosts",
	}

	HooksGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "hooks",
	}

	ForkliftControllersGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "forkliftcontrollers",
	}

	DynamicProvidersGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: "dynamicproviders",
	}

	// SecretGVR is used to access secrets in the cluster
	SecretsGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	// ConfigMapsGVR is used to access configmaps in the cluster
	ConfigMapsGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	// RouteGVR is used to access routes in an Openshift cluster
	RouteGVR = schema.GroupVersionResource{
		Group:    "route.openshift.io",
		Version:  "v1",
		Resource: "routes",
	}
)

// GetDynamicClient returns a dynamic client for interacting with MTV CRDs
func GetDynamicClient(configFlags *genericclioptions.ConfigFlags) (dynamic.Interface, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %v", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	return client, nil
}

// GetKubernetesClientset returns a kubernetes clientset for interacting with the Kubernetes API
func GetKubernetesClientset(configFlags *genericclioptions.ConfigFlags) (*kubernetes.Clientset, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %v", err)
	}

	return clientset, nil
}

// GetAuthenticatedTransport returns an HTTP transport configured with Kubernetes authentication
func GetAuthenticatedTransport(ctx context.Context, configFlags *genericclioptions.ConfigFlags) (http.RoundTripper, error) {
	return GetAuthenticatedTransportWithInsecure(ctx, configFlags, false)
}

// GetAuthenticatedTransportWithInsecure returns an HTTP transport configured with Kubernetes authentication
// and optional insecure TLS skip verification
func GetAuthenticatedTransportWithInsecure(ctx context.Context, configFlags *genericclioptions.ConfigFlags, insecureSkipTLS bool) (http.RoundTripper, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %v", err)
	}

	// Handle client certificate authentication (common in Kind/minikube clusters)
	// The MTV inventory service expects bearer tokens, not client certificates
	if NeedsBearerTokenForInventory(config) {
		klog.V(5).Infof("Detected client certificate authentication without bearer token")

		if token, ok := GetServiceAccountTokenForInventory(ctx, configFlags, config); ok {
			config.BearerToken = token
		} else {
			klog.V(5).Infof("WARNING: Could not retrieve service account token, client certificate auth may not work with inventory service")
		}
	}

	// Debug logging for authentication
	if config.BearerToken != "" {
		klog.V(5).Infof("Using bearer token authentication (token length: %d)", len(config.BearerToken))
	} else if config.BearerTokenFile != "" {
		klog.V(5).Infof("Using bearer token file: %s", config.BearerTokenFile)
	} else if config.CertData != nil || config.CertFile != "" {
		klog.V(5).Infof("Using client certificate authentication")
	} else if config.KeyData != nil || config.KeyFile != "" {
		klog.V(5).Infof("Using client key authentication")
	} else if config.Username != "" {
		klog.V(5).Infof("Using basic authentication with username: %s", config.Username)
	} else if config.ExecProvider != nil {
		klog.V(5).Infof("Using exec auth provider: %s", config.ExecProvider.Command)
	} else if config.AuthProvider != nil {
		klog.V(5).Infof("Using auth provider: %s", config.AuthProvider.Name)
	} else {
		klog.V(5).Infof("WARNING: No authentication credentials found in REST config!")
		klog.V(5).Infof("  BearerToken: %v, CertData: %v, CertFile: %s", config.BearerToken != "", config.CertData != nil, config.CertFile)
		klog.V(5).Infof("  Username: %s, ExecProvider: %v, AuthProvider: %v", config.Username, config.ExecProvider != nil, config.AuthProvider != nil)
	}

	// If insecure skip TLS is enabled, modify the REST config before creating transport
	if insecureSkipTLS {
		config.TLSClientConfig.Insecure = true
		config.TLSClientConfig.CAFile = ""
		config.TLSClientConfig.CAData = nil
		klog.V(5).Infof("TLS certificate verification disabled (insecure mode)")
	}

	// Create a transport wrapper that adds authentication
	// This must be done AFTER modifying the TLS config so the transport is created correctly
	transport, err := rest.TransportFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated transport: %v", err)
	}

	return transport, nil
}

// GetAuthenticatedHTTPClientWithInsecure returns an HTTP client configured with Kubernetes authentication
// and optional insecure TLS skip verification
func GetAuthenticatedHTTPClientWithInsecure(ctx context.Context, configFlags *genericclioptions.ConfigFlags, baseURL string, insecureSkipTLS bool) (*HTTPClient, error) {
	transport, err := GetAuthenticatedTransportWithInsecure(ctx, configFlags, insecureSkipTLS)
	if err != nil {
		return nil, err
	}

	return NewHTTPClient(baseURL, transport), nil
}

// GetAllPlanNames retrieves all plan names from the given namespace
func GetAllPlanNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	var planList *unstructured.UnstructuredList
	if namespace != "" {
		planList, err = dynamicClient.Resource(PlansGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		planList, err = dynamicClient.Resource(PlansGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list plans: %v", err)
	}

	var planNames []string
	for _, plan := range planList.Items {
		planNames = append(planNames, plan.GetName())
	}

	return planNames, nil
}

// GetAllHookNames retrieves all hook names from the given namespace
func GetAllHookNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	var hookList *unstructured.UnstructuredList
	if namespace != "" {
		hookList, err = dynamicClient.Resource(HooksGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		hookList, err = dynamicClient.Resource(HooksGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list hooks: %v", err)
	}

	var hookNames []string
	for _, hook := range hookList.Items {
		hookNames = append(hookNames, hook.GetName())
	}

	return hookNames, nil
}

// GetAllHostNames retrieves all host names from the given namespace
func GetAllHostNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	var hostList *unstructured.UnstructuredList
	if namespace != "" {
		hostList, err = dynamicClient.Resource(HostsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		hostList, err = dynamicClient.Resource(HostsGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %v", err)
	}

	var hostNames []string
	for _, host := range hostList.Items {
		hostNames = append(hostNames, host.GetName())
	}

	return hostNames, nil
}

// GetAllProviderNames retrieves all provider names from the given namespace
func GetAllProviderNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	var providerList *unstructured.UnstructuredList
	if namespace != "" {
		providerList, err = dynamicClient.Resource(ProvidersGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		providerList, err = dynamicClient.Resource(ProvidersGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %v", err)
	}

	var providerNames []string
	for _, provider := range providerList.Items {
		providerNames = append(providerNames, provider.GetName())
	}

	return providerNames, nil
}

// GetAllNetworkMappingNames retrieves all network mapping names from the given namespace
func GetAllNetworkMappingNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	var mappingList *unstructured.UnstructuredList
	if namespace != "" {
		mappingList, err = dynamicClient.Resource(NetworkMapGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		mappingList, err = dynamicClient.Resource(NetworkMapGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list network mappings: %v", err)
	}

	var mappingNames []string
	for _, mapping := range mappingList.Items {
		mappingNames = append(mappingNames, mapping.GetName())
	}

	return mappingNames, nil
}

// GetAllStorageMappingNames retrieves all storage mapping names from the given namespace
func GetAllStorageMappingNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	var mappingList *unstructured.UnstructuredList
	if namespace != "" {
		mappingList, err = dynamicClient.Resource(StorageMapGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		mappingList, err = dynamicClient.Resource(StorageMapGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list storage mappings: %v", err)
	}

	var mappingNames []string
	for _, mapping := range mappingList.Items {
		mappingNames = append(mappingNames, mapping.GetName())
	}

	return mappingNames, nil
}

// HTTPClient represents a client for making HTTP requests with authentication
type HTTPClient struct {
	BaseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a new HTTP client with the given base URL and authentication
func NewHTTPClient(baseURL string, transport http.RoundTripper) *HTTPClient {
	if transport == nil {
		transport = http.DefaultTransport
	}

	client := &http.Client{
		Transport: transport,
	}

	return &HTTPClient{
		BaseURL:    baseURL,
		httpClient: client,
	}
}

// GetWithContext performs a context-aware HTTP GET request to the specified path with credentials.
// This method respects context cancellation and deadlines, making it suitable for long-running
// requests or requests that need to be cancelled (e.g., on SIGINT).
func (c *HTTPClient) GetWithContext(ctx context.Context, path string) ([]byte, error) {
	// Split the path into path part and query part
	parts := strings.SplitN(path, "?", 2)
	pathPart := parts[0]

	// Construct the base URL from baseURL and path part
	fullURL, err := url.JoinPath(c.BaseURL, pathPart)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %v", err)
	}

	// Append query string if it exists
	if len(parts) > 1 {
		fullURL = fullURL + "?" + parts[1]
	}

	// Create a context-aware request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Debug: Log request details (auth is injected by the Kubernetes transport)
	klog.V(5).Infof("Making HTTP request to: %s", fullURL)

	// Execute the request (transport will add auth headers)
	resp, err := c.httpClient.Do(req)

	// Debug: Log response details
	if err == nil {
		klog.V(5).Infof("Response status: %d %s", resp.StatusCode, resp.Status)
	} else {
		klog.V(5).Infof("Request failed: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Check for non-success status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return body, nil
}

// GetDynamicProviderTypes fetches all DynamicProvider types from the cluster.
// Returns an empty slice if the CRD is not available (fails gracefully).
func GetDynamicProviderTypes(configFlags *genericclioptions.ConfigFlags) ([]string, error) {
	dynamicClient, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %v", err)
	}

	ctx := context.Background()

	// Try to list DynamicProvider resources
	list, err := dynamicClient.Resource(DynamicProvidersGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Check if it's a "not found" error (CRD doesn't exist)
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "the server could not find the requested resource") {
			// Fail gracefully - DynamicProvider CRD is not installed
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list DynamicProviders: %v", err)
	}

	// Extract the types from each DynamicProvider
	types := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		providerType, found, err := unstructured.NestedString(item.Object, "spec", "type")
		if err != nil {
			continue // Skip items with errors
		}
		if found && providerType != "" {
			types = append(types, providerType)
		}
	}

	return types, nil
}
