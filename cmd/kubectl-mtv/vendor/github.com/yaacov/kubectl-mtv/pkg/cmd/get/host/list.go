package host

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// extractProviderName gets the provider name from the host spec
func extractProviderName(host unstructured.Unstructured) string {
	provider, found, _ := unstructured.NestedMap(host.Object, "spec", "provider")
	if !found || provider == nil {
		return ""
	}

	if name, ok := provider["name"].(string); ok {
		return name
	}
	return ""
}

// extractHostID gets the host ID from the host spec
func extractHostID(host unstructured.Unstructured) string {
	id, found, _ := unstructured.NestedString(host.Object, "spec", "id")
	if !found {
		return ""
	}
	return id
}

// extractHostIPAddress gets the IP address from the host spec
func extractHostIPAddress(host unstructured.Unstructured) string {
	ip, found, _ := unstructured.NestedString(host.Object, "spec", "ipAddress")
	if !found {
		return ""
	}
	return ip
}

// extractHostStatus gets the status from the host
func extractHostStatus(host unstructured.Unstructured) string {
	ready, found, _ := unstructured.NestedBool(host.Object, "status", "conditions", "Ready")
	if found {
		if ready {
			return "Ready"
		}
		return "Not Ready"
	}

	// Check for conditions array
	conditions, found, _ := unstructured.NestedSlice(host.Object, "status", "conditions")
	if found && len(conditions) > 0 {
		// Look for Ready condition
		for _, condition := range conditions {
			if condMap, ok := condition.(map[string]interface{}); ok {
				if condType, ok := condMap["type"].(string); ok && condType == "Ready" {
					if status, ok := condMap["status"].(string); ok {
						if status == "True" {
							return "Ready"
						}
						return "Not Ready"
					}
				}
			}
		}
	}

	return "Unknown"
}

// createHostItem creates a standardized host item for output
func createHostItem(host unstructured.Unstructured, useUTC bool) map[string]interface{} {
	item := map[string]interface{}{
		"name":      host.GetName(),
		"namespace": host.GetNamespace(),
		"id":        extractHostID(host),
		"provider":  extractProviderName(host),
		"ipAddress": extractHostIPAddress(host),
		"status":    extractHostStatus(host),
		"created":   output.FormatTimestamp(host.GetCreationTimestamp().Time, useUTC),
		"object":    host.Object, // Include the original object
	}

	// Add owner information if available
	if len(host.GetOwnerReferences()) > 0 {
		ownerRef := host.GetOwnerReferences()[0]
		item["owner"] = ownerRef.Name
		item["ownerKind"] = ownerRef.Kind
	}

	return item
}

// List lists hosts
func List(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace, outputFormat string, hostName string, useUTC bool) error {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml", outputFormat)
	}

	var allItems []map[string]interface{}

	// If hostName is specified, get that specific host
	if hostName != "" {
		allItems, err = getSpecificHost(ctx, dynamicClient, namespace, hostName, useUTC)
	} else {
		// Get all hosts
		allItems, err = getAllHosts(ctx, dynamicClient, namespace, useUTC)
	}

	// Handle error if no items found
	if err != nil {
		return err
	}

	// Handle output based on format
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(allItems, "No hosts found.")
	case "yaml":
		return output.PrintYAMLWithEmpty(allItems, "No hosts found.")
	default: // table
		return printHostTable(allItems)
	}
}

// getAllHosts retrieves all hosts from the given namespace
func getAllHosts(ctx context.Context, dynamicClient dynamic.Interface, namespace string, useUTC bool) ([]map[string]interface{}, error) {
	hosts, err := dynamicClient.Resource(client.HostsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %v", err)
	}

	allItems := make([]map[string]interface{}, 0, len(hosts.Items))
	for _, host := range hosts.Items {
		allItems = append(allItems, createHostItem(host, useUTC))
	}

	return allItems, nil
}

// getSpecificHost retrieves a specific host by name
func getSpecificHost(ctx context.Context, dynamicClient dynamic.Interface, namespace string, hostName string, useUTC bool) ([]map[string]interface{}, error) {
	host, err := dynamicClient.Resource(client.HostsGVR).Namespace(namespace).Get(ctx, hostName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get host '%s': %v", hostName, err)
	}

	allItems := []map[string]interface{}{createHostItem(*host, useUTC)}
	return allItems, nil
}

// printHostTable prints hosts in table format
func printHostTable(items []map[string]interface{}) error {
	if len(items) == 0 {
		fmt.Println("No hosts found.")
		return nil
	}

	// Create table headers
	headers := []string{"NAME", "ID", "PROVIDER", "IP ADDRESS", "STATUS", "CREATED"}

	// Prepare table data
	var data [][]string
	for _, item := range items {
		row := []string{
			fmt.Sprintf("%v", item["name"]),
			fmt.Sprintf("%v", item["id"]),
			fmt.Sprintf("%v", item["provider"]),
			fmt.Sprintf("%v", item["ipAddress"]),
			fmt.Sprintf("%v", item["status"]),
			fmt.Sprintf("%v", item["created"]),
		}
		data = append(data, row)
	}

	// Print the table using TablePrinter
	printer := output.NewTablePrinter()

	// Create headers using Header struct
	var tableHeaders []output.Header
	for _, header := range headers {
		tableHeaders = append(tableHeaders, output.Header{
			DisplayName: header,
			JSONPath:    strings.ToLower(strings.ReplaceAll(header, " ", "")),
		})
	}

	printer.WithHeaders(tableHeaders...)

	// Convert data to map format for the table printer
	for _, row := range data {
		item := map[string]interface{}{
			"name":      row[0],
			"id":        row[1],
			"provider":  row[2],
			"ipaddress": row[3],
			"status":    row[4],
			"created":   row[5],
		}
		printer.AddItem(item)
	}

	return printer.Print()
}
