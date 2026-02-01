package inventory

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListResourcePoolsWithInsecure queries the provider's resource pool inventory with optional insecure TLS skip verification
func ListResourcePoolsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listResourcePoolsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listResourcePoolsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listResourcePoolsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify resource pool support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers based on provider type
	var defaultHeaders []output.Header
	switch providerType {
	case "vsphere":
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "CPU_LIMIT", JSONPath: "cpuLimit"},
			{DisplayName: "CPU_SHARES", JSONPath: "cpuShares"},
			{DisplayName: "MEM_LIMIT", JSONPath: "memoryLimitFormatted"},
			{DisplayName: "MEM_SHARES", JSONPath: "memoryShares"},
			{DisplayName: "REVISION", JSONPath: "revision"},
		}
	default:
		return fmt.Errorf("provider type '%s' does not support resource pool inventory", providerType)
	}

	// Fetch resource pools inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "vsphere":
		data, err = providerClient.GetResourcePools(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support resource pool inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch resource pool inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for resource pool inventory")
	}

	// Convert to expected format and add calculated fields
	resourcePools := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		resourcePool, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Add human-readable memory limit formatting
		if memoryLimit, exists := resourcePool["memoryLimit"]; exists {
			if memoryLimitFloat, ok := memoryLimit.(float64); ok {
				resourcePool["memoryLimitFormatted"] = humanizeBytes(memoryLimitFloat)
			}
		}

		resourcePools = append(resourcePools, resourcePool)
	}

	// Parse query options for advanced query features
	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}

		// Apply query filter
		filteredData, err := querypkg.ApplyQueryInterface(resourcePools, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
		// Convert back to []map[string]interface{}
		if convertedData, ok := filteredData.([]interface{}); ok {
			resourcePools = make([]map[string]interface{}, 0, len(convertedData))
			for _, item := range convertedData {
				if resourcePoolMap, ok := item.(map[string]interface{}); ok {
					resourcePools = append(resourcePools, resourcePoolMap)
				}
			}
		}
	}

	// Format and display the results
	emptyMessage := fmt.Sprintf("No resource pools found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(resourcePools, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(resourcePools, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(resourcePools, defaultHeaders, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
