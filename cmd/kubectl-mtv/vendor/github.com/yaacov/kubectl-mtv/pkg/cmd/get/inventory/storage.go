package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListStorage queries the provider's storage inventory and displays the results
func ListStorage(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listStorageOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listStorageOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listStorageOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to determine which storage resource to fetch
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
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "DEFAULT", JSONPath: "object.metadata.annotations[storageclass.kubernetes.io/is-default-class]"},
			{DisplayName: "VIRT-DEFAULT", JSONPath: "object.metadata.annotations[storageclass.kubevirt.io/is-default-virt-class]"},
		}
	default:
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "TYPE", JSONPath: "type"},
			{DisplayName: "CAPACITY", JSONPath: "capacityHuman"},
			{DisplayName: "FREE", JSONPath: "freeHuman"},
			{DisplayName: "MAINTENANCE", JSONPath: "maintenance"},
		}
	}

	// Fetch storage inventory based on provider type
	var data interface{}
	switch providerType {
	case "ovirt":
		data, err = providerClient.GetStorageDomains(4)
	case "vsphere":
		data, err = providerClient.GetDatastores(4)
	case "ova":
		data, err = providerClient.GetResourceCollection("storages", 4)
	case "openstack":
		data, err = providerClient.GetVolumeTypes(4)
	case "openshift":
		data, err = providerClient.GetStorageClasses(4)
	default:
		// For other providers, use generic storage resource
		data, err = providerClient.GetResourceCollection("storages", 4)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Convert to expected format
	storages := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if storage, ok := item.(map[string]interface{}); ok {
			// Add provider name to each storage
			storage["provider"] = providerName

			// Humanize capacity and free space
			if capacity, exists := storage["capacity"]; exists {
				if capacityFloat, ok := capacity.(float64); ok {
					storage["capacityHuman"] = humanizeBytes(capacityFloat)
				} else if capacityNum, ok := capacity.(int64); ok {
					storage["capacityHuman"] = humanizeBytes(float64(capacityNum))
				}
			}

			if free, exists := storage["free"]; exists {
				if freeFloat, ok := free.(float64); ok {
					storage["freeHuman"] = humanizeBytes(freeFloat)
				} else if freeNum, ok := free.(int64); ok {
					storage["freeHuman"] = humanizeBytes(float64(freeNum))
				}
			}

			storages = append(storages, storage)
		}
	}

	// Parse and apply query options
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	// Apply query options (sorting, filtering, limiting)
	storages, err = querypkg.ApplyQuery(storages, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml", outputFormat)
	}

	// Handle different output formats
	emptyMessage := fmt.Sprintf("No storage resources found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(storages, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(storages, emptyMessage)
	default:
		return output.PrintTableWithQuery(storages, defaultHeaders, queryOpts, emptyMessage)
	}
}
