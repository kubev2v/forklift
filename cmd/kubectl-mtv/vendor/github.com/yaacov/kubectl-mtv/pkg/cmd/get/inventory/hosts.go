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

// ListHostsWithInsecure queries the provider's host inventory with optional insecure TLS skip verification
func ListHostsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listHostsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listHostsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listHostsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify host support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Fetch hosts inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "ovirt", "vsphere":
		data, err = providerClient.GetHosts(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support host inventory", providerType)
	}

	// Error handling
	if err != nil {
		return fmt.Errorf("failed to fetch host inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for host inventory")
	}

	// Convert to expected format
	hosts := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if host, ok := item.(map[string]interface{}); ok {
			// Add provider name to each host
			host["provider"] = providerName
			hosts = append(hosts, host)
		}
	}

	// Parse and apply query options
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	// Apply query options (sorting, filtering, limiting)
	hosts, err = querypkg.ApplyQuery(hosts, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml", outputFormat)
	}

	// Handle different output formats
	emptyMessage := fmt.Sprintf("No hosts found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(hosts, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(hosts, emptyMessage)
	default:
		// Define default headers
		defaultHeaders := []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "STATUS", JSONPath: "status"},
			{DisplayName: "VERSION", JSONPath: "productVersion"},
			{DisplayName: "MGMT IP", JSONPath: "managementServerIp"},
			{DisplayName: "CORES", JSONPath: "cpuCores"},
			{DisplayName: "SOCKETS", JSONPath: "cpuSockets"},
			{DisplayName: "MAINTENANCE", JSONPath: "inMaintenance"},
		}
		return output.PrintTableWithQuery(hosts, defaultHeaders, queryOpts, emptyMessage)
	}
}
