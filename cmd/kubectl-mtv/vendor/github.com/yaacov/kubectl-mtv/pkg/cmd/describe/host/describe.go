package host

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe describes a migration host
func Describe(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, useUTC bool) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the host
	host, err := c.Resource(client.HostsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get host: %v", err)
	}

	// Print the host details
	fmt.Printf("\n%s", output.ColorizedSeparator(105, output.YellowColor))
	fmt.Printf("\n%s\n", output.Bold("MIGRATION HOST"))

	// Basic Information
	fmt.Printf("%s %s\n", output.Bold("Name:"), output.Yellow(host.GetName()))
	fmt.Printf("%s %s\n", output.Bold("Namespace:"), output.Yellow(host.GetNamespace()))
	fmt.Printf("%s %s\n", output.Bold("Created:"), output.Yellow(output.FormatTimestamp(host.GetCreationTimestamp().Time, useUTC)))

	// Host Spec Information
	if hostID, found, _ := unstructured.NestedString(host.Object, "spec", "id"); found {
		fmt.Printf("%s %s\n", output.Bold("Host ID:"), output.Yellow(hostID))
	}

	if hostName, found, _ := unstructured.NestedString(host.Object, "spec", "name"); found {
		fmt.Printf("%s %s\n", output.Bold("Host Name:"), output.Yellow(hostName))
	}

	if ipAddress, found, _ := unstructured.NestedString(host.Object, "spec", "ipAddress"); found {
		fmt.Printf("%s %s\n", output.Bold("IP Address:"), output.Yellow(ipAddress))
	}

	// Provider Information
	if providerMap, found, _ := unstructured.NestedMap(host.Object, "spec", "provider"); found {
		if providerName, ok := providerMap["name"].(string); ok {
			fmt.Printf("%s %s\n", output.Bold("Provider:"), output.Yellow(providerName))
		}
	}

	// Secret Information
	if secretMap, found, _ := unstructured.NestedMap(host.Object, "spec", "secret"); found {
		if secretName, ok := secretMap["name"].(string); ok {
			fmt.Printf("%s %s\n", output.Bold("Secret:"), output.Yellow(secretName))
		}
	}

	// Owner References
	if len(host.GetOwnerReferences()) > 0 {
		fmt.Printf("\n%s\n", output.Bold("OWNERSHIP"))
		for _, owner := range host.GetOwnerReferences() {
			fmt.Printf("%s %s/%s", output.Bold("Owner:"), owner.Kind, owner.Name)
			if owner.Controller != nil && *owner.Controller {
				fmt.Printf(" %s", output.Green("(controller)"))
			}
			fmt.Println()
		}
	}

	// Network Adapters Information from Provider Inventory
	if err := displayNetworkAdapters(ctx, configFlags, host, namespace); err != nil {
		// Log the error but don't fail the command - network adapter info is supplementary
		fmt.Printf("\n%s: %v\n", output.Bold("Network Adapters Info"), output.Red("Failed to fetch"))
	}

	// Status Information
	if status, found, _ := unstructured.NestedMap(host.Object, "status"); found && status != nil {
		fmt.Printf("\n%s\n", output.Bold("STATUS"))

		// Conditions
		if conditions, found, _ := unstructured.NestedSlice(host.Object, "status", "conditions"); found {
			fmt.Printf("%s\n", output.Bold("Conditions:"))
			for _, condition := range conditions {
				if condMap, ok := condition.(map[string]interface{}); ok {
					condType, _ := condMap["type"].(string)
					condStatus, _ := condMap["status"].(string)
					reason, _ := condMap["reason"].(string)
					message, _ := condMap["message"].(string)
					lastTransitionTime, _ := condMap["lastTransitionTime"].(string)

					fmt.Printf("  %s: %s", output.Bold(condType), output.ColorizeStatus(condStatus))
					if reason != "" {
						fmt.Printf(" (%s)", reason)
					}
					fmt.Println()

					if message != "" {
						fmt.Printf("    %s\n", message)
					}
					if lastTransitionTime != "" {
						fmt.Printf("    Last Transition: %s\n", lastTransitionTime)
					}
				}
			}
		}

		// Other status fields
		if observedGeneration, found, _ := unstructured.NestedInt64(host.Object, "status", "observedGeneration"); found {
			fmt.Printf("%s %d\n", output.Bold("Observed Generation:"), observedGeneration)
		}
	}

	// Annotations
	if annotations := host.GetAnnotations(); len(annotations) > 0 {
		fmt.Printf("\n%s\n", output.Bold("ANNOTATIONS"))
		for key, value := range annotations {
			fmt.Printf("%s: %s\n", output.Bold(key), value)
		}
	}

	// Labels
	if labels := host.GetLabels(); len(labels) > 0 {
		fmt.Printf("\n%s\n", output.Bold("LABELS"))
		for key, value := range labels {
			fmt.Printf("%s: %s\n", output.Bold(key), value)
		}
	}

	fmt.Println() // Add a newline at the end
	return nil
}

// displayNetworkAdapters fetches and displays network adapter information from provider inventory
func displayNetworkAdapters(ctx context.Context, configFlags *genericclioptions.ConfigFlags, host *unstructured.Unstructured, namespace string) error {
	// Extract host ID and provider name from host resource
	hostID, found, _ := unstructured.NestedString(host.Object, "spec", "id")
	if !found || hostID == "" {
		return fmt.Errorf("host ID not found in host spec")
	}

	providerMap, found, _ := unstructured.NestedMap(host.Object, "spec", "provider")
	if !found {
		return fmt.Errorf("provider information not found in host spec")
	}

	providerName, ok := providerMap["name"].(string)
	if !ok || providerName == "" {
		return fmt.Errorf("provider name not found in host spec")
	}

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return fmt.Errorf("failed to get provider: %v", err)
	}

	// Create provider client with inventory URL discovery
	inventoryURL := client.DiscoverInventoryURL(ctx, configFlags, namespace)
	providerClient := inventory.NewProviderClient(configFlags, provider, inventoryURL)

	// Get provider type to verify host support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Only fetch network adapters for supported provider types
	if providerType != "ovirt" && providerType != "vsphere" {
		return fmt.Errorf("provider type '%s' does not support host inventory", providerType)
	}

	// Fetch specific host data from provider inventory
	hostData, err := providerClient.GetHost(hostID, 4) // detail level 4 for full info
	if err != nil {
		return fmt.Errorf("failed to fetch host inventory data: %v", err)
	}

	// Extract network adapters from host data
	hostMap, ok := hostData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected host data format")
	}

	networkAdapters, found, _ := unstructured.NestedSlice(hostMap, "networkAdapters")
	if !found || len(networkAdapters) == 0 {
		return fmt.Errorf("no network adapters found")
	}

	// Display network adapters information
	fmt.Printf("\n%s\n", output.Bold("NETWORK ADAPTERS"))
	for i, adapter := range networkAdapters {
		if adapterMap, ok := adapter.(map[string]interface{}); ok {
			fmt.Printf("%s %d:\n", output.Bold("Adapter"), i+1)

			if name, ok := adapterMap["name"].(string); ok {
				fmt.Printf("  %s %s\n", output.Bold("Name:"), output.Yellow(name))
			}

			if ipAddress, ok := adapterMap["ipAddress"].(string); ok {
				fmt.Printf("  %s %s\n", output.Bold("IP Address:"), output.Yellow(ipAddress))
			}

			if subnetMask, ok := adapterMap["subnetMask"].(string); ok {
				fmt.Printf("  %s %s\n", output.Bold("Subnet Mask:"), output.Yellow(subnetMask))
			}

			if mtu, ok := adapterMap["mtu"].(float64); ok {
				fmt.Printf("  %s %.0f\n", output.Bold("MTU:"), mtu)
			}

			if linkSpeed, ok := adapterMap["linkSpeed"].(float64); ok {
				fmt.Printf("  %s %.0f Mbps\n", output.Bold("Link Speed:"), linkSpeed)
			}

			// Add spacing between adapters if there are multiple
			if i < len(networkAdapters)-1 {
				fmt.Println()
			}
		}
	}

	return nil
}
