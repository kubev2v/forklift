package inventory

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListProvidersWithInsecure queries the providers and displays their inventory information with optional insecure TLS skip verification
func ListProvidersWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listProvidersOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listProvidersOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listProvidersOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// If inventoryURL is empty, try to discover it from an OpenShift Route
	if inventoryURL == "" {
		inventoryURL = client.DiscoverInventoryURL(ctx, kubeConfigFlags, namespace)
	}

	if inventoryURL == "" {
		return fmt.Errorf("inventory URL not provided and could not be discovered")
	}

	// Fetch provider inventory data directly from inventory API with detail=4
	var providersData interface{}
	var err error

	if providerName != "" {
		// Get specific provider by name with detail=4
		providersData, err = client.FetchSpecificProviderWithDetailAndInsecure(ctx, kubeConfigFlags, inventoryURL, providerName, 4, insecureSkipTLS)
		if err != nil {
			return fmt.Errorf("failed to get provider inventory: %v", err)
		}
	} else {
		// Get all providers with detail=4
		providersData, err = client.FetchProvidersWithDetailAndInsecure(ctx, kubeConfigFlags, inventoryURL, 4, insecureSkipTLS)
		if err != nil {
			return fmt.Errorf("failed to fetch providers inventory: %v", err)
		}
	}

	// Parse provider inventory data
	var items []map[string]interface{}
	if providersMap, ok := providersData.(map[string]interface{}); ok {
		// Iterate through all provider types (vsphere, ovirt, openstack, etc.)
		for providerType, providerList := range providersMap {
			if providerListSlice, ok := providerList.([]interface{}); ok {
				for _, p := range providerListSlice {
					if providerMap, ok := p.(map[string]interface{}); ok {
						// Create item directly from inventory data (no CRD needed)
						item := map[string]interface{}{
							"type": providerType,
						}

						// Copy all fields from inventory data
						for key, value := range providerMap {
							item[key] = value
						}

						// Add some derived fields for compatibility
						if namespace != "" {
							item["namespace"] = namespace
						}

						items = append(items, item)
					}
				}
			}
		}
	} else {
		return fmt.Errorf("unexpected provider inventory data format")
	}

	// Parse and apply query options
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	// Apply query options (sorting, filtering, limiting)
	items, err = querypkg.ApplyQuery(items, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml", outputFormat)
	}

	// Handle different output formats
	emptyMessage := "No providers found"
	if namespace != "" {
		emptyMessage = fmt.Sprintf("No providers found in namespace %s", namespace)
	}

	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(items, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(items, emptyMessage)
	default:
		// Define headers optimized for inventory information
		defaultHeaders := []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
		}

		// Add NAMESPACE column when listing across all namespaces (only if namespace data is available)
		if namespace == "" {
			// Only add namespace column if any items have namespace info
			hasNamespace := false
			for _, item := range items {
				if _, exists := item["namespace"]; exists {
					hasNamespace = true
					break
				}
			}
			if hasNamespace {
				defaultHeaders = append(defaultHeaders, output.Header{DisplayName: "NAMESPACE", JSONPath: "namespace"})
			}
		}

		// Add remaining columns focused on inventory
		defaultHeaders = append(defaultHeaders,
			output.Header{DisplayName: "TYPE", JSONPath: "type"},
			output.Header{DisplayName: "VERSION", JSONPath: "apiVersion"},
			output.Header{DisplayName: "PHASE", JSONPath: "object.status.phase"},
			output.Header{DisplayName: "VMS", JSONPath: "vmCount"},
			output.Header{DisplayName: "HOSTS", JSONPath: "hostCount"},
		)

		return output.PrintTableWithQuery(items, defaultHeaders, queryOpts, emptyMessage)
	}
}
