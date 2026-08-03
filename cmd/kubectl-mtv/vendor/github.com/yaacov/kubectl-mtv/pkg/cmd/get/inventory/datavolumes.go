package inventory

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListDataVolumesWithInsecure queries the provider's data volume inventory with optional insecure TLS skip verification
func ListDataVolumesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	sq := watch.NewSafeQuery(query)

	return watch.WrapWithWatchAndQuery(watchMode, outputFormat, func() error {
		return listDataVolumesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, sq.Get(), insecureSkipTLS)
	}, watch.DefaultInterval, sq.Set, query)
}

func listDataVolumesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify data volume support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers based on provider type
	var defaultHeaders []output.Column
	switch providerType {
	case "openshift":
		defaultHeaders = []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "NAMESPACE", Key: "namespace"},
			{Title: "ID", Key: "id"},
			{Title: "PHASE", Key: "object.status.phase", ColorFunc: output.ColorizeStatus},
			{Title: "PROGRESS", Key: "object.status.progress", ColorFunc: output.ColorizeProgress},
			{Title: "STORAGE_CLASS", Key: "object.spec.pvc.storageClassName"},
			{Title: "SIZE", Key: "sizeFormatted"},
			{Title: "CREATED", Key: "object.metadata.creationTimestamp"},
		}
	default:
		return fmt.Errorf("provider type '%s' does not support data volume inventory", providerType)
	}

	// Fetch data volume inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "openshift":
		data, err = providerClient.GetDataVolumes(ctx, 4)
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
	case "markdown":
		return output.PrintMarkdownWithQuery(dataVolumes, defaultHeaders, queryOpts, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(dataVolumes, defaultHeaders, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
