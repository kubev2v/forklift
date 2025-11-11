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

// ListDataCenters queries the provider's datacenter inventory and displays the results
func ListDataCenters(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listDataCentersOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listDataCentersOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listDataCentersOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to verify datacenter support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "DESCRIPTION", JSONPath: "description"},
		{DisplayName: "STATUS", JSONPath: "status"},
	}

	// Fetch datacenters inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "ovirt", "vsphere":
		data, err = providerClient.GetDataCenters(4)
	default:
		return fmt.Errorf("provider type '%s' does not support datacenter inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get datacenters from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No datacenters found for provider %s", providerName)
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
