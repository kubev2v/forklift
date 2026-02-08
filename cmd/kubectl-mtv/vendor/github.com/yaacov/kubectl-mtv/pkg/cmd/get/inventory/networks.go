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

// countNetworkHosts calculates the number of hosts connected to a network
func countNetworkHosts(network map[string]interface{}) int {
	hosts, exists := network["host"]
	if !exists {
		return 0
	}

	hostsArray, ok := hosts.([]interface{})
	if !ok {
		return 0
	}

	return len(hostsArray)
}

// countNetworkSubnets calculates the number of subnets in a network (for OpenStack)
func countNetworkSubnets(network map[string]interface{}) int {
	subnets, exists := network["subnets"]
	if !exists {
		return 0
	}

	subnetsArray, ok := subnets.([]interface{})
	if !ok {
		return 0
	}

	return len(subnetsArray)
}

// ListNetworksWithInsecure queries the provider's network inventory and displays the results with optional insecure TLS skip verification
func ListNetworksWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listNetworksOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listNetworksOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listNetworksOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to determine resource path and headers
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
			{DisplayName: "CREATED", JSONPath: "object.metadata.creationTimestamp"},
		}
	case "openstack":
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "STATUS", JSONPath: "status"},
			{DisplayName: "SHARED", JSONPath: "shared"},
			{DisplayName: "ADMIN-UP", JSONPath: "adminStateUp"},
			{DisplayName: "SUBNETS", JSONPath: "subnetsCount"},
		}
	case "ec2":
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "TYPE", JSONPath: "networkType"},
			{DisplayName: "CIDR", JSONPath: "CidrBlock"},
			{DisplayName: "STATE", JSONPath: "State"},
			{DisplayName: "DEFAULT", JSONPath: "IsDefault"},
		}
	default:
		defaultHeaders = []output.Header{
			{DisplayName: "NAME", JSONPath: "name"},
			{DisplayName: "ID", JSONPath: "id"},
			{DisplayName: "VARIANT", JSONPath: "variant"},
			{DisplayName: "HOSTS", JSONPath: "hostCount"},
			{DisplayName: "VLAN", JSONPath: "vlanId"},
			{DisplayName: "REVISION", JSONPath: "revision"},
		}
	}

	// Fetch network inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "openshift":
		// For OpenShift, get network attachment definitions
		data, err = providerClient.GetResourceCollection(ctx, "networkattachmentdefinitions", 4)
	default:
		// For other providers, get networks
		data, err = providerClient.GetNetworks(ctx, 4)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch network inventory: %v", err)
	}

	// Extract objects from EC2 envelope
	if providerType == "ec2" {
		data = ExtractEC2Objects(data)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for network inventory")
	}

	// Convert to expected format
	networks := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if network, ok := item.(map[string]interface{}); ok {
			// Add provider name to each network
			network["provider"] = providerName

			// Add host count (for ovirt, vsphere, etc.)
			network["hostCount"] = countNetworkHosts(network)

			// Add subnets count (for OpenStack)
			if providerType == "openstack" {
				network["subnetsCount"] = countNetworkSubnets(network)
			}

			// Process EC2 networks (extract name from tags, set ID and type)
			if providerType == "ec2" {
				processEC2Network(network)
			}

			networks = append(networks, network)
		}
	}

	// Parse and apply query options
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	// Apply query options (sorting, filtering, limiting)
	networks, err = querypkg.ApplyQuery(networks, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml", outputFormat)
	}

	// Handle different output formats
	emptyMessage := fmt.Sprintf("No networks found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(networks, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(networks, emptyMessage)
	default:
		return output.PrintTableWithQuery(networks, defaultHeaders, queryOpts, emptyMessage)
	}
}
