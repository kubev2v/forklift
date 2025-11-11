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

// ListDisks queries the provider's disk inventory and displays the results
func ListDisks(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listDisksOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listDisksOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listDisksOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to verify disk support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers based on provider type
	var defaultHeaders []output.Header
	switch providerType {
	case "ova":
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "PATH", JSONPath: "path"},
			{DisplayName: "SIZE", JSONPath: "sizeHuman"},
			{DisplayName: "VM-COUNT", JSONPath: "vmCount"},
		}
	default:
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "STORAGE-DOMAIN", JSONPath: "storageDomain.name"},
			{DisplayName: "SIZE", JSONPath: "provisionedSizeHuman"},
			{DisplayName: "ACTUAL-SIZE", JSONPath: "actualSizeHuman"},
			{DisplayName: "TYPE", JSONPath: "storageType"},
			{DisplayName: "STATUS", JSONPath: "status"},
		}
	}

	// Fetch disks inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "ovirt":
		data, err = providerClient.GetDisks(4)
	case "openstack":
		data, err = providerClient.GetVolumes(4)
	case "ova":
		data, err = providerClient.GetOVAFiles(4)
	default:
		return fmt.Errorf("provider type '%s' does not support disk inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get disks from provider: %v", err)
	}

	// Process data to add human-readable sizes for oVirt
	if providerType == "ovirt" {
		data = addHumanReadableSizes(data)
	}

	// Process data to add human-readable sizes for oVirt
	if providerType == "ova" {
		data = addHumanReadableOVASizes(data)
	}

	// Parse query options for advanced query features
	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}

		// Apply query filter
		data, err = querypkg.ApplyQueryInterface(data, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
	}

	// Format and display the results
	emptyMessage := fmt.Sprintf("No disks found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(data, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(data, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(data, defaultHeaders, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

// addHumanReadableSizes adds human-readable size fields to disk data
func addHumanReadableSizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if disk, ok := item.(map[string]interface{}); ok {
				if provisionedSize, exists := disk["provisionedSize"]; exists {
					if size, ok := provisionedSize.(float64); ok {
						disk["provisionedSizeHuman"] = humanizeBytes(size)
					}
				}
				if actualSize, exists := disk["actualSize"]; exists {
					if size, ok := actualSize.(float64); ok {
						disk["actualSizeHuman"] = humanizeBytes(size)
					}
				}
			}
		}
	case map[string]interface{}:
		if provisionedSize, exists := v["provisionedSize"]; exists {
			if size, ok := provisionedSize.(float64); ok {
				v["provisionedSizeHuman"] = humanizeBytes(size)
			}
		}
		if actualSize, exists := v["actualSize"]; exists {
			if size, ok := actualSize.(float64); ok {
				v["actualSizeHuman"] = humanizeBytes(size)
			}
		}
	}
	return data
}

// addHumanReadableOVASizes adds human-readable size fields to OVA data
func addHumanReadableOVASizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if ova, ok := item.(map[string]interface{}); ok {
				if size, exists := ova["size"]; exists {
					if sizeVal, ok := size.(float64); ok {
						ova["sizeHuman"] = humanizeBytes(sizeVal)
					}
				}
			}
		}
	case map[string]interface{}:
		if size, exists := v["size"]; exists {
			if sizeVal, ok := size.(float64); ok {
				v["sizeHuman"] = humanizeBytes(sizeVal)
			}
		}
	}
	return data
}
