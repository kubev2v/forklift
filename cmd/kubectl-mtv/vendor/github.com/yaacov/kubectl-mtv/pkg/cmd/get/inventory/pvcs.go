package inventory

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListPersistentVolumeClaimsWithInsecure queries the provider's persistent volume claim inventory with optional insecure TLS skip verification
func ListPersistentVolumeClaimsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listPersistentVolumeClaimsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listPersistentVolumeClaimsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listPersistentVolumeClaimsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify PVC support
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
			{DisplayName: "STATUS", JSONPath: "object.status.phase"},
			{DisplayName: "CAPACITY", JSONPath: "object.status.capacity.storage"},
			{DisplayName: "STORAGE_CLASS", JSONPath: "object.spec.storageClassName"},
			{DisplayName: "ACCESS_MODES", JSONPath: "object.spec.accessModes"},
			{DisplayName: "CREATED", JSONPath: "object.metadata.creationTimestamp"},
		}
	default:
		return fmt.Errorf("provider type '%s' does not support persistent volume claim inventory", providerType)
	}

	// Fetch PVC inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "openshift":
		data, err = providerClient.GetPersistentVolumeClaims(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support persistent volume claim inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch persistent volume claim inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for persistent volume claim inventory")
	}

	// Convert to expected format and add calculated fields
	pvcs := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		pvc, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Add human-readable capacity formatting using GetValueByPathString
		if storage, err := querypkg.GetValueByPathString(pvc, "object.status.capacity.storage"); err == nil {
			if storageStr, ok := storage.(string); ok {
				pvc["capacityFormatted"] = storageStr
			}
		}

		// Format access modes using GetValueByPathString
		if accessModes, err := querypkg.GetValueByPathString(pvc, "object.spec.accessModes"); err == nil {
			if accessModesArray, ok := accessModes.([]interface{}); ok {
				var modes []string
				for _, mode := range accessModesArray {
					if modeStr, ok := mode.(string); ok {
						modes = append(modes, modeStr)
					}
				}
				pvc["accessModes"] = modes
			}
		}

		pvcs = append(pvcs, pvc)
	}

	// Parse query options for advanced query features
	var queryOpts *querypkg.QueryOptions
	if query != "" {
		queryOpts, err = querypkg.ParseQueryString(query)
		if err != nil {
			return fmt.Errorf("failed to parse query: %v", err)
		}

		// Apply query filter
		filteredData, err := querypkg.ApplyQueryInterface(pvcs, query)
		if err != nil {
			return fmt.Errorf("failed to apply query: %v", err)
		}
		// Convert back to []map[string]interface{}
		if convertedData, ok := filteredData.([]interface{}); ok {
			pvcs = make([]map[string]interface{}, 0, len(convertedData))
			for _, item := range convertedData {
				if pvcMap, ok := item.(map[string]interface{}); ok {
					pvcs = append(pvcs, pvcMap)
				}
			}
		}
	}

	// Format and display the results
	emptyMessage := fmt.Sprintf("No persistent volume claims found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(pvcs, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(pvcs, emptyMessage)
	case "table":
		return output.PrintTableWithQuery(pvcs, defaultHeaders, queryOpts, emptyMessage)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
