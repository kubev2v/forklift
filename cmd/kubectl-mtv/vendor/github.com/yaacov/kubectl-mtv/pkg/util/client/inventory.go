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

// FetchProvidersWithDetail fetches lists of providers from the inventory server with specified detail level
func FetchProvidersWithDetail(configFlags *genericclioptions.ConfigFlags, baseURL string, detail int) (interface{}, error) {
	httpClient, err := GetAuthenticatedHTTPClient(configFlags, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %v", err)
	}

	// Construct the path for provider inventory with detail level
	path := fmt.Sprintf("/providers?detail=%d", detail)

	klog.V(4).Infof("Fetching provider inventory from: %s%s", baseURL, path)

	// Fetch the provider inventory
	responseBytes, err := httpClient.Get(path)
	if err != nil {
		return nil, err
	}

	// Parse the response as JSON
	var result interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse provider inventory response: %v", err)
	}

	return result, nil
}

// FetchProviders fetches lists of providers from the inventory server (detail level 1 for backward compatibility)
func FetchProviders(configFlags *genericclioptions.ConfigFlags, baseURL string) (interface{}, error) {
	return FetchProvidersWithDetail(configFlags, baseURL, 1)
}

// FetchProviderInventory fetches inventory for a specific provider
func FetchProviderInventory(configFlags *genericclioptions.ConfigFlags, baseURL string, provider *unstructured.Unstructured, subPath string) (interface{}, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}

	httpClient, err := GetAuthenticatedHTTPClient(configFlags, baseURL)
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

	klog.V(4).Infof("Fetching provider inventory from path: %s", path)

	// Fetch the provider inventory
	responseBytes, err := httpClient.Get(path)
	if err != nil {
		return nil, err
	}

	// Parse the response as JSON
	var result interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse provider inventory response: %v", err)
	}

	return result, nil
}

// FetchSpecificProvider fetches inventory for a specific provider by name (detail level 1 for backward compatibility)
func FetchSpecificProvider(ctx context.Context, configFlags *genericclioptions.ConfigFlags, baseURL string, providerName string) (interface{}, error) {
	return FetchSpecificProviderWithDetail(ctx, configFlags, baseURL, providerName, 1)
}

// FetchSpecificProviderWithDetail fetches inventory for a specific provider by name with specified detail level
// This function uses direct URL access: /providers/<type>/<uid>?detail=N
func FetchSpecificProviderWithDetail(ctx context.Context, configFlags *genericclioptions.ConfigFlags, baseURL string, providerName string, detail int) (interface{}, error) {
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
	httpClient, err := GetAuthenticatedHTTPClient(configFlags, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %v", err)
	}

	path := fmt.Sprintf("/providers/%s/%s?detail=%d", url.PathEscape(providerType), url.PathEscape(providerUID), detail)
	klog.V(4).Infof("Fetching specific provider inventory from path: %s", path)

	// Fetch the provider inventory
	responseBytes, err := httpClient.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provider inventory: %v", err)
	}

	// Parse the response as JSON
	var result interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse provider inventory response: %v", err)
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
