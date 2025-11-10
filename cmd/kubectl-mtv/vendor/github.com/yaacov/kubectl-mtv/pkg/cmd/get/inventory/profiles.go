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

// ListDiskProfiles queries the provider's disk profile inventory and displays the results
func ListDiskProfiles(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listDiskProfilesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listDiskProfilesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listDiskProfilesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to verify disk profile support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "STORAGE-DOMAIN", JSONPath: "storageDomain.name"},
		{DisplayName: "QOS", JSONPath: "qos.name"},
	}

	// Fetch disk profiles inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "ovirt":
		data, err = providerClient.GetDiskProfiles(4)
	default:
		return fmt.Errorf("provider type '%s' does not support disk profile inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get disk profiles from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No disk profiles found for provider %s", providerName)
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

// ListNICProfiles queries the provider's NIC profile inventory and displays the results
func ListNICProfiles(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listNICProfilesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
		}, 10*time.Second)
	}

	return listNICProfilesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query)
}

func listNICProfilesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClient(kubeConfigFlags, provider, inventoryURL)

	// Get provider type to verify NIC profile support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "NETWORK", JSONPath: "network.name"},
		{DisplayName: "PORT-MIRRORING", JSONPath: "portMirroring"},
		{DisplayName: "PASS-THROUGH", JSONPath: "passThrough"},
		{DisplayName: "QOS", JSONPath: "qos.name"},
	}

	// Fetch NIC profiles inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "ovirt":
		data, err = providerClient.GetNICProfiles(4)
	default:
		return fmt.Errorf("provider type '%s' does not support NIC profile inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get NIC profiles from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No NIC profiles found for provider %s", providerName)
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
