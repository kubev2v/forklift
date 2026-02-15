package inventory

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListNamespacesWithInsecure queries the provider's namespace inventory with optional insecure TLS skip verification
func ListNamespacesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listNamespacesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listNamespacesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listNamespacesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify namespace support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Fetch namespace inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "openshift":
		data, err = providerClient.GetNamespaces(ctx, 4)
	case "openstack":
		data, err = providerClient.GetProjects(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support namespace inventory", providerType)
	}

	// Error handling
	if err != nil {
		return fmt.Errorf("failed to fetch namespace inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for namespace inventory")
	}

	// Convert to expected format
	namespaces := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if ns, ok := item.(map[string]interface{}); ok {
			// Add provider name to each namespace
			ns["provider"] = providerName
			namespaces = append(namespaces, ns)
		}
	}

	// Parse and apply query options
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	// Apply query options (sorting, filtering, limiting)
	namespaces, err = querypkg.ApplyQuery(namespaces, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml", outputFormat)
	}

	// Handle different output formats
	emptyMessage := fmt.Sprintf("No namespaces found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(namespaces, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(namespaces, emptyMessage)
	default:
		// Define default headers
		defaultHeaders := []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "PROVIDER", JSONPath: "provider"},
		}
		return output.PrintTableWithQuery(namespaces, defaultHeaders, queryOpts, emptyMessage)
	}
}
