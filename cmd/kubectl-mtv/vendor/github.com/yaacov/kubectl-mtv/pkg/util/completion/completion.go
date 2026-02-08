package completion

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// getResourceNames fetches resource names for completion
func getResourceNames(ctx context.Context, configFlags *genericclioptions.ConfigFlags, gvr schema.GroupVersionResource, namespace string) ([]string, error) {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, err
	}

	var list []string

	// List resources
	if namespace != "" {
		resources, err := c.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, resource := range resources.Items {
			list = append(list, resource.GetName())
		}
	} else {
		resources, err := c.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, resource := range resources.Items {
			list = append(list, resource.GetName())
		}
	}

	return list, nil
}

// PlanNameCompletion provides completion for plan names
func PlanNameCompletion(configFlags *genericclioptions.ConfigFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace := client.ResolveNamespace(configFlags)

		names, err := getResourceNames(context.Background(), configFlags, client.PlansGVR, namespace)
		if err != nil {
			return []string{fmt.Sprintf("Error fetching plans: %v", err)}, cobra.ShellCompDirectiveError
		}

		if len(names) == 0 {
			namespaceMsg := "current namespace"
			if namespace != "" {
				namespaceMsg = fmt.Sprintf("namespace '%s'", namespace)
			}
			return []string{fmt.Sprintf("No migration plans found in %s", namespaceMsg)}, cobra.ShellCompDirectiveError
		}

		// Filter results based on what's already typed
		var filtered []string
		for _, name := range names {
			if strings.HasPrefix(name, toComplete) {
				filtered = append(filtered, name)
			}
		}

		if len(filtered) == 0 && toComplete != "" {
			return []string{fmt.Sprintf("No migration plans matching '%s'", toComplete)}, cobra.ShellCompDirectiveError
		}

		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}

// ProviderNameCompletion provides completion for provider names
func ProviderNameCompletion(configFlags *genericclioptions.ConfigFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return ProviderNameCompletionByType(configFlags, "")
}

// ProviderNameCompletionByType provides completion for provider names filtered by provider type
// If providerType is empty, returns all providers
func ProviderNameCompletionByType(configFlags *genericclioptions.ConfigFlags, providerType string) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace := client.ResolveNamespace(configFlags)

		// Get all providers
		c, err := client.GetDynamicClient(configFlags)
		if err != nil {
			return []string{fmt.Sprintf("Error getting client: %v", err)}, cobra.ShellCompDirectiveError
		}

		resources, err := c.Resource(client.ProvidersGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return []string{fmt.Sprintf("Error fetching providers: %v", err)}, cobra.ShellCompDirectiveError
		}

		if len(resources.Items) == 0 {
			namespaceMsg := "current namespace"
			if namespace != "" {
				namespaceMsg = fmt.Sprintf("namespace '%s'", namespace)
			}
			return []string{fmt.Sprintf("No providers found in %s", namespaceMsg)}, cobra.ShellCompDirectiveError
		}

		// Filter providers by type if specified
		var filtered []string
		for _, resource := range resources.Items {
			resourceName := resource.GetName()

			// Skip if doesn't match what's being typed
			if !strings.HasPrefix(resourceName, toComplete) {
				continue
			}

			// If provider type filter is specified, check the provider type
			if providerType != "" {
				resourceType, found, err := unstructured.NestedString(resource.Object, "spec", "type")
				if err != nil || !found || resourceType != providerType {
					continue
				}
			}

			filtered = append(filtered, resourceName)
		}

		if len(filtered) == 0 {
			if providerType != "" {
				if toComplete != "" {
					return []string{fmt.Sprintf("No %s providers matching '%s'", providerType, toComplete)}, cobra.ShellCompDirectiveError
				}
				return []string{fmt.Sprintf("No %s providers found", providerType)}, cobra.ShellCompDirectiveError
			} else {
				if toComplete != "" {
					return []string{fmt.Sprintf("No providers matching '%s'", toComplete)}, cobra.ShellCompDirectiveError
				}
				return []string{"No providers found"}, cobra.ShellCompDirectiveError
			}
		}

		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}

// MappingNameCompletion provides completion for mapping names
// mappingType should be "network" or "storage"
func MappingNameCompletion(configFlags *genericclioptions.ConfigFlags, mappingType string) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace := client.ResolveNamespace(configFlags)

		var gvr schema.GroupVersionResource
		var resourceType string
		if mappingType == "storage" {
			gvr = client.StorageMapGVR
			resourceType = "storage mappings"
		} else {
			gvr = client.NetworkMapGVR
			resourceType = "network mappings"
		}

		names, err := getResourceNames(context.Background(), configFlags, gvr, namespace)
		if err != nil {
			return []string{fmt.Sprintf("Error fetching %s: %v", resourceType, err)}, cobra.ShellCompDirectiveError
		}

		if len(names) == 0 {
			namespaceMsg := "current namespace"
			if namespace != "" {
				namespaceMsg = fmt.Sprintf("namespace '%s'", namespace)
			}
			return []string{fmt.Sprintf("No %s found in %s", resourceType, namespaceMsg)}, cobra.ShellCompDirectiveError
		}

		// Filter results based on what's already typed
		var filtered []string
		for _, name := range names {
			if strings.HasPrefix(name, toComplete) {
				filtered = append(filtered, name)
			}
		}

		if len(filtered) == 0 && toComplete != "" {
			return []string{fmt.Sprintf("No %s matching '%s'", resourceType, toComplete)}, cobra.ShellCompDirectiveError
		}

		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}

// HostNameCompletion provides completion for host names from provider inventory
func HostNameCompletion(configFlags *genericclioptions.ConfigFlags, providerName, toComplete string) ([]string, cobra.ShellCompDirective) {
	if providerName == "" {
		return []string{"Provider not specified"}, cobra.ShellCompDirectiveError
	}

	namespace := client.ResolveNamespace(configFlags)
	inventoryURL := client.DiscoverInventoryURL(context.Background(), configFlags, namespace)

	// Validate provider is vSphere
	provider, err := inventory.GetProviderByName(context.Background(), configFlags, providerName, namespace)
	if err != nil {
		return []string{fmt.Sprintf("Error getting provider: %v", err)}, cobra.ShellCompDirectiveError
	}

	// Check if it's a vSphere provider
	providerType, found, err := unstructured.NestedString(provider.Object, "spec", "type")
	if err != nil || !found {
		return []string{fmt.Sprintf("Error getting provider type: %v", err)}, cobra.ShellCompDirectiveError
	}

	if providerType != "vsphere" {
		return []string{fmt.Sprintf("Only vSphere providers support hosts, got: %s", providerType)}, cobra.ShellCompDirectiveError
	}

	// Get available hosts
	// Note: Completion functions use insecure=false as a safe default
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, false)
	data, err := providerClient.GetHosts(context.Background(), 4)
	if err != nil {
		return []string{fmt.Sprintf("Error fetching hosts: %v", err)}, cobra.ShellCompDirectiveError
	}

	// Convert to expected format
	dataArray, ok := data.([]interface{})
	if !ok {
		return []string{"Error: unexpected data format for host inventory"}, cobra.ShellCompDirectiveError
	}

	// Extract host IDs
	var hostIDs []string
	for _, item := range dataArray {
		if host, ok := item.(map[string]interface{}); ok {
			if id, ok := host["id"].(string); ok {
				if strings.HasPrefix(id, toComplete) {
					hostIDs = append(hostIDs, id)
				}
			}
		}
	}

	if len(hostIDs) == 0 {
		if toComplete != "" {
			return []string{fmt.Sprintf("No host IDs matching '%s'", toComplete)}, cobra.ShellCompDirectiveError
		}
		return []string{"No host IDs found"}, cobra.ShellCompDirectiveError
	}

	return hostIDs, cobra.ShellCompDirectiveNoFileComp
}

// HostIPAddressCompletion provides completion for IP addresses from host network adapters
func HostIPAddressCompletion(configFlags *genericclioptions.ConfigFlags, providerName string, hostNames []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if providerName == "" {
		return []string{"Provider not specified"}, cobra.ShellCompDirectiveError
	}

	if len(hostNames) == 0 {
		return []string{"No hosts specified"}, cobra.ShellCompDirectiveError
	}

	namespace := client.ResolveNamespace(configFlags)
	inventoryURL := client.DiscoverInventoryURL(context.Background(), configFlags, namespace)

	// Get provider
	provider, err := inventory.GetProviderByName(context.Background(), configFlags, providerName, namespace)
	if err != nil {
		return []string{fmt.Sprintf("Error getting provider: %v", err)}, cobra.ShellCompDirectiveError
	}

	// Get available hosts
	// Note: Completion functions use insecure=false as a safe default
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, false)
	data, err := providerClient.GetHosts(context.Background(), 4)
	if err != nil {
		return []string{fmt.Sprintf("Error fetching hosts: %v", err)}, cobra.ShellCompDirectiveError
	}

	dataArray, ok := data.([]interface{})
	if !ok {
		return []string{"Error: unexpected data format for host inventory"}, cobra.ShellCompDirectiveError
	}

	// Extract IP addresses from specified hosts' network adapters
	var ipAddresses []string
	seenIPs := make(map[string]bool)

	for _, item := range dataArray {
		if host, ok := item.(map[string]interface{}); ok {
			if hostID, ok := host["id"].(string); ok {
				// Check if this host is in our list
				hostInList := false
				for _, requestedHost := range hostNames {
					if hostID == requestedHost {
						hostInList = true
						break
					}
				}
				if !hostInList {
					continue
				}

				// Extract IP addresses from network adapters
				if networkAdapters, ok := host["networkAdapters"].([]interface{}); ok {
					for _, adapter := range networkAdapters {
						if adapterMap, ok := adapter.(map[string]interface{}); ok {
							if ipAddress, ok := adapterMap["ipAddress"].(string); ok && ipAddress != "" {
								if strings.HasPrefix(ipAddress, toComplete) && !seenIPs[ipAddress] {
									ipAddresses = append(ipAddresses, ipAddress)
									seenIPs[ipAddress] = true
								}
							}
						}
					}
				}
			}
		}
	}

	if len(ipAddresses) == 0 {
		if toComplete != "" {
			return []string{fmt.Sprintf("No IP addresses matching '%s' found for specified hosts", toComplete)}, cobra.ShellCompDirectiveError
		}
		return []string{"No IP addresses found for specified hosts"}, cobra.ShellCompDirectiveError
	}

	return ipAddresses, cobra.ShellCompDirectiveNoFileComp
}

// HostNetworkAdapterCompletion provides completion for network adapter names from host inventory
func HostNetworkAdapterCompletion(configFlags *genericclioptions.ConfigFlags, providerName string, hostNames []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if providerName == "" {
		return []string{"Provider not specified"}, cobra.ShellCompDirectiveError
	}

	if len(hostNames) == 0 {
		return []string{"No hosts specified"}, cobra.ShellCompDirectiveError
	}

	namespace := client.ResolveNamespace(configFlags)
	inventoryURL := client.DiscoverInventoryURL(context.Background(), configFlags, namespace)

	// Get provider
	provider, err := inventory.GetProviderByName(context.Background(), configFlags, providerName, namespace)
	if err != nil {
		return []string{fmt.Sprintf("Error getting provider: %v", err)}, cobra.ShellCompDirectiveError
	}

	// Get available hosts
	// Note: Completion functions use insecure=false as a safe default
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, false)
	data, err := providerClient.GetHosts(context.Background(), 4)
	if err != nil {
		return []string{fmt.Sprintf("Error fetching hosts: %v", err)}, cobra.ShellCompDirectiveError
	}

	dataArray, ok := data.([]interface{})
	if !ok {
		return []string{"Error: unexpected data format for host inventory"}, cobra.ShellCompDirectiveError
	}

	// Extract network adapter names from specified hosts
	var adapterNames []string
	seenAdapters := make(map[string]bool)

	for _, item := range dataArray {
		if host, ok := item.(map[string]interface{}); ok {
			if hostID, ok := host["id"].(string); ok {
				// Check if this host is in our list
				hostInList := false
				for _, requestedHost := range hostNames {
					if hostID == requestedHost {
						hostInList = true
						break
					}
				}
				if !hostInList {
					continue
				}

				// Extract adapter names from network adapters
				if networkAdapters, ok := host["networkAdapters"].([]interface{}); ok {
					for _, adapter := range networkAdapters {
						if adapterMap, ok := adapter.(map[string]interface{}); ok {
							if adapterName, ok := adapterMap["name"].(string); ok && adapterName != "" {
								if strings.HasPrefix(adapterName, toComplete) && !seenAdapters[adapterName] {
									adapterNames = append(adapterNames, adapterName)
									seenAdapters[adapterName] = true
								}
							}
						}
					}
				}
			}
		}
	}

	if len(adapterNames) == 0 {
		if toComplete != "" {
			return []string{fmt.Sprintf("No network adapters matching '%s' found for specified hosts", toComplete)}, cobra.ShellCompDirectiveError
		}
		return []string{"No network adapters found for specified hosts"}, cobra.ShellCompDirectiveError
	}

	return adapterNames, cobra.ShellCompDirectiveNoFileComp
}

// MigrationNameCompletion provides completion for migration names
func MigrationNameCompletion(configFlags *genericclioptions.ConfigFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace := client.ResolveNamespace(configFlags)

		names, err := getResourceNames(context.Background(), configFlags, client.MigrationsGVR, namespace)
		if err != nil {
			return []string{fmt.Sprintf("Error fetching migrations: %v", err)}, cobra.ShellCompDirectiveError
		}

		if len(names) == 0 {
			namespaceMsg := "current namespace"
			if namespace != "" {
				namespaceMsg = fmt.Sprintf("namespace '%s'", namespace)
			}
			return []string{fmt.Sprintf("No migrations found in %s", namespaceMsg)}, cobra.ShellCompDirectiveError
		}

		// Filter results based on what's already typed
		var filtered []string
		for _, name := range names {
			if strings.HasPrefix(name, toComplete) {
				filtered = append(filtered, name)
			}
		}

		if len(filtered) == 0 && toComplete != "" {
			return []string{fmt.Sprintf("No migrations matching '%s'", toComplete)}, cobra.ShellCompDirectiveError
		}

		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}

// HookResourceNameCompletion provides completion for hook resource names
func HookResourceNameCompletion(configFlags *genericclioptions.ConfigFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace := client.ResolveNamespace(configFlags)

		names, err := getResourceNames(context.Background(), configFlags, client.HooksGVR, namespace)
		if err != nil {
			return []string{fmt.Sprintf("Error fetching hooks: %v", err)}, cobra.ShellCompDirectiveError
		}

		if len(names) == 0 {
			namespaceMsg := "current namespace"
			if namespace != "" {
				namespaceMsg = fmt.Sprintf("namespace '%s'", namespace)
			}
			return []string{fmt.Sprintf("No hook resources found in %s", namespaceMsg)}, cobra.ShellCompDirectiveError
		}

		// Filter results based on what's already typed
		var filtered []string
		for _, name := range names {
			if strings.HasPrefix(name, toComplete) {
				filtered = append(filtered, name)
			}
		}

		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}

// HostResourceNameCompletion provides completion for host resource names
func HostResourceNameCompletion(configFlags *genericclioptions.ConfigFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace := client.ResolveNamespace(configFlags)

		names, err := getResourceNames(context.Background(), configFlags, client.HostsGVR, namespace)
		if err != nil {
			return []string{fmt.Sprintf("Error fetching hosts: %v", err)}, cobra.ShellCompDirectiveError
		}

		if len(names) == 0 {
			namespaceMsg := "current namespace"
			if namespace != "" {
				namespaceMsg = fmt.Sprintf("namespace '%s'", namespace)
			}
			return []string{fmt.Sprintf("No host resources found in %s", namespaceMsg)}, cobra.ShellCompDirectiveError
		}

		// Filter results based on what's already typed
		var filtered []string
		for _, name := range names {
			if strings.HasPrefix(name, toComplete) {
				filtered = append(filtered, name)
			}
		}

		if len(filtered) == 0 && toComplete != "" {
			return []string{fmt.Sprintf("No host resources matching '%s'", toComplete)}, cobra.ShellCompDirectiveError
		}

		return filtered, cobra.ShellCompDirectiveNoFileComp
	}
}
