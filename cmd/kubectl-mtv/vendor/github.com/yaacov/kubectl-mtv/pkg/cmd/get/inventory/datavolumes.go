package inventory

import (
	"context"
	"fmt"
	"time"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListDataVolumes queries the provider's data volume inventory and displays the results
func ListDataVolumes(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listDataVolumesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listDataVolumesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listDataVolumesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to verify data volume support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers based on provider type
	var defaultHeaders []output.Header
	switch providerType {
	case "openshift":
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "NAMESPACE", JSONPath: "namespace"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "PHASE", JSONPath: "object.status.phase"},
			{DisplayName: "PROGRESS", JSONPath: "object.status.progress"},
			{DisplayName: "STORAGE_CLASS", JSONPath: "object.spec.pvc.storageClassName"},
			{DisplayName: "SIZE", JSONPath: "sizeFormatted"},
			{DisplayName: "CREATED", JSONPath: "object.metadata.creationTimestamp"},
		}
	default:
		return fmt.Errorf("provider type '%s' does not support data volume inventory", providerType)
	}

	// Fetch data volume inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "openshift":
		data, err = providerClient.GetDataVolumes(4)
	default:
		return fmt.Errorf("provider type '%s' does not support data volume inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch data volume inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for data volume inventory")
	}

	// Convert to expected format and add calculated fields
	dataVolumes := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		dataVolume, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Add human-readable size formatting using GetValueByPathString
		if storage, err := querypkg.GetValueByPathString(dataVolume, "object.spec.pvc.resources.requests.storage"); err == nil {
			if storageStr, ok := storage.(string); ok {
				dataVolume["sizeFormatted"] = storageStr
			}
		}

		dataVolumes = append(dataVolumes, dataVolume)
	}

	// Parse query options for advanced query features
	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}

		// Apply query filter
		filteredData, err := querypkg.ApplyQueryInterface(dataVolumes, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
		// Convert back to []map[string]interface{}
		if convertedData, ok := filteredData.([]interface{}); ok {
			dataVolumes = make([]map[string]interface{}, 0, len(convertedData))
			for _, item := range convertedData {
				if dataVolumeMap, ok := item.(map[string]interface{}); ok {
					dataVolumes = append(dataVolumes, dataVolumeMap)
				}
			}
		}
	}

	// Format and display the results
	emptyMessage := fmt.Sprintf("No data volumes found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(dataVolumes, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(dataVolumes, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(dataVolumes, defaultHeaders, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
