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

// ListFolders queries the provider's folder inventory and displays the results
func ListFolders(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listFoldersOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listFoldersOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listFoldersOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to verify folder support
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
			{DisplayName: "PARENT", JSONPath: "parent"},
			{DisplayName: "PATH", JSONPath: "path"},
			{DisplayName: "DATACENTER", JSONPath: "datacenter"},
			{DisplayName: "REVISION", JSONPath: "revision"},
		}
	default:
		return fmt.Errorf("provider type '%s' does not support folder inventory", providerType)
	}

	// Fetch folders inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "vsphere":
		data, err = providerClient.GetFolders(4)
	default:
		return fmt.Errorf("provider type '%s' does not support folder inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch folder inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for folder inventory")
	}

	// Convert to expected format
	folders := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		folder, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		folders = append(folders, folder)
	}

	// Parse query options for advanced query features
	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}

		// Apply query filter
		filteredData, err := querypkg.ApplyQueryInterface(folders, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
		// Convert back to []map[string]interface{}
		if convertedData, ok := filteredData.([]interface{}); ok {
			folders = make([]map[string]interface{}, 0, len(convertedData))
			for _, item := range convertedData {
				if folderMap, ok := item.(map[string]interface{}); ok {
					folders = append(folders, folderMap)
				}
			}
		}
	}

	// Generate output
	// Format and display the results
	emptyMessage := fmt.Sprintf("No folders found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(folders, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(folders, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(folders, defaultHeaders, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
