package inventory

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListDatastoresWithInsecure queries the provider's datastore inventory with optional insecure TLS skip verification
func ListDatastoresWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listDatastoresOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listDatastoresOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listDatastoresOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify datastore support
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
			{DisplayName: "TYPE", JSONPath: "type"},
			{DisplayName: "CAPACITY", JSONPath: "capacityFormatted"},
			{DisplayName: "FREE", JSONPath: "freeSpaceFormatted"},
			{DisplayName: "ACCESSIBLE", JSONPath: "accessible"},
			{DisplayName: "REVISION", JSONPath: "revision"},
		}
	default:
		return fmt.Errorf("provider type '%s' does not support datastore inventory", providerType)
	}

	// Fetch datastores inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "vsphere":
		data, err = providerClient.GetDatastores(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support datastore inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch datastore inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for datastore inventory")
	}

	// Convert to expected format and add calculated fields
	datastores := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		datastore, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Add human-readable capacity formatting
		if capacity, exists := datastore["capacity"]; exists {
			if capacityFloat, ok := capacity.(float64); ok {
				datastore["capacityFormatted"] = humanizeBytes(capacityFloat)
			}
		}

		// Add human-readable free space formatting
		if freeSpace, exists := datastore["freeSpace"]; exists {
			if freeSpaceFloat, ok := freeSpace.(float64); ok {
				datastore["freeSpaceFormatted"] = humanizeBytes(freeSpaceFloat)
			}
		}

		datastores = append(datastores, datastore)
	}

	// Parse query options for advanced query features
	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}

		// Apply query filter
		filteredData, err := querypkg.ApplyQueryInterface(datastores, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
		// Convert back to []map[string]interface{}
		if convertedData, ok := filteredData.([]interface{}); ok {
			datastores = make([]map[string]interface{}, 0, len(convertedData))
			for _, item := range convertedData {
				if datastoreMap, ok := item.(map[string]interface{}); ok {
					datastores = append(datastores, datastoreMap)
				}
			}
		}
	}

	// Generate output
	emptyMessage := fmt.Sprintf("No datastores found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(datastores, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(datastores, emptyMessage)
	default:
		return output.PrintTableWithQuery(datastores, defaultHeaders, queryOpts, emptyMessage)
	}
}
