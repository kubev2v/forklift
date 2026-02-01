package inventory

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// ListInstancesWithInsecure queries the provider's instance inventory with optional insecure TLS skip verification
func ListInstancesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listInstancesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listInstancesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listInstancesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify instance support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "STATUS", JSONPath: "status"},
		{DisplayName: "FLAVOR", JSONPath: "flavor.name"},
		{DisplayName: "IMAGE", JSONPath: "image.name"},
		{DisplayName: "PROJECT", JSONPath: "project.name"},
	}

	// Fetch instances inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetInstances(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support instance inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get instances from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No instances found for provider %s", providerName)
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

// ListImagesWithInsecure queries the provider's image inventory with optional insecure TLS skip verification
func ListImagesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listImagesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listImagesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listImagesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify image support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "STATUS", JSONPath: "status"},
		{DisplayName: "SIZE", JSONPath: "sizeHuman"},
		{DisplayName: "VISIBILITY", JSONPath: "visibility"},
	}

	// Fetch images inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetImages(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support image inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get images from provider: %v", err)
	}

	// Process data to add human-readable sizes
	data = addHumanReadableImageSizes(data)

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
	emptyMessage := fmt.Sprintf("No images found for provider %s", providerName)
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

// ListFlavorsWithInsecure queries the provider's flavor inventory with optional insecure TLS skip verification
func ListFlavorsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listFlavorsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listFlavorsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listFlavorsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify flavor support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "VCPUS", JSONPath: "vcpus"},
		{DisplayName: "RAM", JSONPath: "ramHuman"},
		{DisplayName: "DISK", JSONPath: "diskHuman"},
		{DisplayName: "EPHEMERAL", JSONPath: "ephemeralHuman"},
	}

	// Fetch flavors inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetFlavors(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support flavor inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get flavors from provider: %v", err)
	}

	// Process data to add human-readable sizes
	data = addHumanReadableFlavorSizes(data)

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
	emptyMessage := fmt.Sprintf("No flavors found for provider %s", providerName)
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

// ListProjects queries the provider's project inventory and displays the results
// ListProjectsWithInsecure queries the provider's project inventory with optional insecure TLS skip verification
func ListProjectsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listProjectsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listProjectsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listProjectsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify project support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "DESCRIPTION", JSONPath: "description"},
		{DisplayName: "ENABLED", JSONPath: "enabled"},
	}

	// Fetch projects inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetProjects(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support project inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get projects from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No projects found for provider %s", providerName)
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

// ListVolumesWithInsecure queries the provider's volume inventory with optional insecure TLS skip verification
func ListVolumesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listVolumesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listVolumesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listVolumesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify volume support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "STATUS", JSONPath: "status"},
		{DisplayName: "SIZE", JSONPath: "sizeHuman"},
		{DisplayName: "TYPE", JSONPath: "volumeType"},
		{DisplayName: "BOOTABLE", JSONPath: "bootable"},
	}

	// Fetch volumes inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetVolumes(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support volume inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get volumes from provider: %v", err)
	}

	// Process data to add human-readable sizes
	data = addHumanReadableVolumeSizes(data)

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
	emptyMessage := fmt.Sprintf("No volumes found for provider %s", providerName)
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

// ListVolumeTypesWithInsecure queries the provider's volume type inventory with optional insecure TLS skip verification
func ListVolumeTypesWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listVolumeTypesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listVolumeTypesOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listVolumeTypesOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify volume type support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "DESCRIPTION", JSONPath: "description"},
		{DisplayName: "PUBLIC", JSONPath: "isPublic"},
	}

	// Fetch volume types inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetVolumeTypes(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support volume type inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get volume types from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No volume types found for provider %s", providerName)
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

// ListSnapshotsWithInsecure queries the provider's snapshot inventory with optional insecure TLS skip verification
func ListSnapshotsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listSnapshotsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listSnapshotsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listSnapshotsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify snapshot support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "STATUS", JSONPath: "status"},
		{DisplayName: "SIZE", JSONPath: "sizeHuman"},
		{DisplayName: "VOLUME-ID", JSONPath: "volumeID"},
	}

	// Fetch snapshots inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetSnapshots(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support snapshot inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get snapshots from provider: %v", err)
	}

	// Process data to add human-readable sizes
	data = addHumanReadableSnapshotSizes(data)

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
	emptyMessage := fmt.Sprintf("No snapshots found for provider %s", providerName)
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

// ListSubnetsWithInsecure queries the provider's subnet inventory with optional insecure TLS skip verification
func ListSubnetsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listSubnetsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
		}, watch.DefaultInterval)
	}

	return listSubnetsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, query, insecureSkipTLS)
}

func listSubnetsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify subnet support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Define default headers
	defaultHeaders := []output.Header{
		{DisplayName: "NAME", JSONPath: "name"},
		{DisplayName: "ID", JSONPath: "id"},
		{DisplayName: "NETWORK-ID", JSONPath: "networkID"},
		{DisplayName: "CIDR", JSONPath: "cidr"},
		{DisplayName: "IP-VERSION", JSONPath: "ipVersion"},
		{DisplayName: "GATEWAY", JSONPath: "gatewayIP"},
		{DisplayName: "DHCP", JSONPath: "enableDHCP"},
	}

	// Fetch subnets inventory from the provider
	var data interface{}
	switch providerType {
	case "openstack":
		data, err = providerClient.GetSubnets(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support subnet inventory", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to get subnets from provider: %v", err)
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
	emptyMessage := fmt.Sprintf("No subnets found for provider %s", providerName)
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

// Helper functions for adding human-readable sizes

// addHumanReadableImageSizes adds human-readable size fields to image data
func addHumanReadableImageSizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if image, ok := item.(map[string]interface{}); ok {
				if size, exists := image["size"]; exists {
					if sizeVal, ok := size.(float64); ok {
						image["sizeHuman"] = humanizeBytes(sizeVal)
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

// addHumanReadableFlavorSizes adds human-readable size fields to flavor data
func addHumanReadableFlavorSizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if flavor, ok := item.(map[string]interface{}); ok {
				if ram, exists := flavor["ram"]; exists {
					if ramVal, ok := ram.(float64); ok {
						flavor["ramHuman"] = humanizeBytes(ramVal * 1024 * 1024) // RAM is in MB
					}
				}
				if disk, exists := flavor["disk"]; exists {
					if diskVal, ok := disk.(float64); ok {
						flavor["diskHuman"] = humanizeBytes(diskVal * 1024 * 1024 * 1024) // Disk is in GB
					}
				}
				if ephemeral, exists := flavor["ephemeral"]; exists {
					if ephemeralVal, ok := ephemeral.(float64); ok {
						flavor["ephemeralHuman"] = humanizeBytes(ephemeralVal * 1024 * 1024 * 1024) // Ephemeral is in GB
					}
				}
			}
		}
	case map[string]interface{}:
		if ram, exists := v["ram"]; exists {
			if ramVal, ok := ram.(float64); ok {
				v["ramHuman"] = humanizeBytes(ramVal * 1024 * 1024) // RAM is in MB
			}
		}
		if disk, exists := v["disk"]; exists {
			if diskVal, ok := disk.(float64); ok {
				v["diskHuman"] = humanizeBytes(diskVal * 1024 * 1024 * 1024) // Disk is in GB
			}
		}
		if ephemeral, exists := v["ephemeral"]; exists {
			if ephemeralVal, ok := ephemeral.(float64); ok {
				v["ephemeralHuman"] = humanizeBytes(ephemeralVal * 1024 * 1024 * 1024) // Ephemeral is in GB
			}
		}
	}
	return data
}

// addHumanReadableVolumeSizes adds human-readable size fields to volume data
func addHumanReadableVolumeSizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if volume, ok := item.(map[string]interface{}); ok {
				if size, exists := volume["size"]; exists {
					if sizeVal, ok := size.(float64); ok {
						volume["sizeHuman"] = humanizeBytes(sizeVal * 1024 * 1024 * 1024) // Size is in GB
					}
				}
			}
		}
	case map[string]interface{}:
		if size, exists := v["size"]; exists {
			if sizeVal, ok := size.(float64); ok {
				v["sizeHuman"] = humanizeBytes(sizeVal * 1024 * 1024 * 1024) // Size is in GB
			}
		}
	}
	return data
}

// addHumanReadableSnapshotSizes adds human-readable size fields to snapshot data
func addHumanReadableSnapshotSizes(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if snapshot, ok := item.(map[string]interface{}); ok {
				if size, exists := snapshot["size"]; exists {
					if sizeVal, ok := size.(float64); ok {
						snapshot["sizeHuman"] = humanizeBytes(sizeVal * 1024 * 1024 * 1024) // Size is in GB
					}
				}
			}
		}
	case map[string]interface{}:
		if size, exists := v["size"]; exists {
			if sizeVal, ok := size.(float64); ok {
				v["sizeHuman"] = humanizeBytes(sizeVal * 1024 * 1024 * 1024) // Size is in GB
			}
		}
	}
	return data
}
