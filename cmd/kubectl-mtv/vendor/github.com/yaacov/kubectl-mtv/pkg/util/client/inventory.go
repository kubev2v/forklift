package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// parseJSONResponse parses a JSON response, treating empty or null responses as empty arrays.
// For malformed JSON, it provides a helpful error message with a preview of the response.
func parseJSONResponse(responseBytes []byte) (interface{}, error) {
	// Handle empty response as empty array (not an error)
	if len(responseBytes) == 0 {
		return []interface{}{}, nil
	}

	// Parse the response as JSON
	var result interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		// Provide more context for debugging malformed responses
		preview := string(responseBytes)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		return nil, fmt.Errorf("failed to parse inventory response as JSON: %v (response preview: %q)", err, preview)
	}

	// Handle JSON null as empty array
	if result == nil {
		return []interface{}{}, nil
	}

	return result, nil
}

// FetchProvidersWithDetailAndInsecure fetches lists of providers from the inventory server with specified detail level
// and optional insecure TLS skip verification
func FetchProvidersWithDetailAndInsecure(ctx context.Context, configFlags *genericclioptions.ConfigFlags, baseURL string, detail int, insecureSkipTLS bool) (interface{}, error) {
	httpClient, err := GetAuthenticatedHTTPClientWithInsecure(ctx, configFlags, baseURL, insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %v", err)
	}

	// Construct the path for provider inventory with detail level
	path := fmt.Sprintf("/providers?detail=%d", detail)

	klog.V(4).Infof("Fetching provider inventory from: %s%s (insecure=%v)", baseURL, path, insecureSkipTLS)

	// Fetch the provider inventory
	responseBytes, err := httpClient.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}

	return parseJSONResponse(responseBytes)
}

// FetchProviderInventoryWithInsecure fetches inventory for a specific provider with optional insecure TLS skip verification
func FetchProviderInventoryWithInsecure(ctx context.Context, configFlags *genericclioptions.ConfigFlags, baseURL string, provider *unstructured.Unstructured, subPath string, insecureSkipTLS bool) (interface{}, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}

	httpClient, err := GetAuthenticatedHTTPClientWithInsecure(ctx, configFlags, baseURL, insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %v", err)
	}

	providerType, found, err := unstructured.NestedString(provider.Object, "spec", "type")
	if err != nil || !found {
		return nil, fmt.Errorf("provider type not found or error retrieving it: %v", err)
	}

	providerUID, found, err := unstructured.NestedString(provider.Object, "metadata", "uid")
	if err != nil || !found {
		return nil, fmt.Errorf("provider UID not found or error retrieving it: %v", err)
	}

	// Construct the path for provider inventory: /providers/<spec.type>/<metadata.uid>
	path := fmt.Sprintf("/providers/%s/%s", url.PathEscape(providerType), url.PathEscape(providerUID))

	// Add subPath if provided
	if subPath != "" {
		path = fmt.Sprintf("%s/%s", path, strings.TrimPrefix(subPath, "/"))
	}

	klog.V(4).Infof("Fetching provider inventory from path: %s (insecure=%v)", path, insecureSkipTLS)

	// Fetch the provider inventory
	responseBytes, err := httpClient.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}

	return parseJSONResponse(responseBytes)
}

// FetchSpecificProviderWithDetailAndInsecure fetches inventory for a specific provider by name with specified detail level
// and optional insecure TLS skip verification
// This function uses direct URL access: /providers/<type>/<uid>?detail=N
func FetchSpecificProviderWithDetailAndInsecure(ctx context.Context, configFlags *genericclioptions.ConfigFlags, baseURL string, providerName string, detail int, insecureSkipTLS bool) (interface{}, error) {
	// We need to determine the namespace to look for the provider CRD
	// Try to get it from configFlags or use empty string for all namespaces
	namespace := ""
	if configFlags.Namespace != nil && *configFlags.Namespace != "" {
		namespace = *configFlags.Namespace
	}

	// First get the provider CRD by name to extract type and UID
	c, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	var provider *unstructured.Unstructured
	if namespace != "" {
		// Get from specific namespace
		provider, err = c.Resource(ProvidersGVR).Namespace(namespace).Get(ctx, providerName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get provider '%s' in namespace '%s': %v", providerName, namespace, err)
		}
	} else {
		// Search all namespaces
		providersList, err := c.Resource(ProvidersGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list providers: %v", err)
		}

		var foundProvider *unstructured.Unstructured
		for _, p := range providersList.Items {
			if p.GetName() == providerName {
				foundProvider = &p
				break
			}
		}

		if foundProvider == nil {
			return nil, fmt.Errorf("provider '%s' not found", providerName)
		}
		provider = foundProvider
	}

	// Extract provider type and UID from the CRD
	providerType, found, err := unstructured.NestedString(provider.Object, "spec", "type")
	if err != nil || !found {
		return nil, fmt.Errorf("provider type not found or error retrieving it: %v", err)
	}

	providerUID := string(provider.GetUID())
	if providerUID == "" {
		return nil, fmt.Errorf("provider UID not found")
	}

	// Use direct URL to fetch provider inventory: /providers/<type>/<uid>?detail=N
	httpClient, err := GetAuthenticatedHTTPClientWithInsecure(ctx, configFlags, baseURL, insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %v", err)
	}

	path := fmt.Sprintf("/providers/%s/%s?detail=%d", url.PathEscape(providerType), url.PathEscape(providerUID), detail)
	klog.V(4).Infof("Fetching specific provider inventory from path: %s (insecure=%v)", path, insecureSkipTLS)

	// Fetch the provider inventory
	responseBytes, err := httpClient.GetWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provider inventory: %v", err)
	}

	result, err := parseJSONResponse(responseBytes)
	if err != nil {
		return nil, err
	}

	// Wrap the result in the same structure as FetchProviders for consistency
	return map[string]interface{}{
		providerType: []interface{}{result},
	}, nil
}

// DiscoverInventoryURL tries to discover the inventory URL from an OpenShift Route
func DiscoverInventoryURL(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) string {
	route, err := GetForkliftInventoryRoute(ctx, configFlags, namespace)
	if err == nil && route != nil {
		host, found, _ := unstructured.NestedString(route.Object, "spec", "host")
		if found && host != "" {
			return fmt.Sprintf("https://%s", host)
		}
	}
	return ""
}
