package hook

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// extractHookImage gets the image from the hook spec
func extractHookImage(hook unstructured.Unstructured) string {
	image, found, _ := unstructured.NestedString(hook.Object, "spec", "image")
	if !found {
		return ""
	}
	return image
}

// extractHookServiceAccount gets the service account from the hook spec
func extractHookServiceAccount(hook unstructured.Unstructured) string {
	serviceAccount, found, _ := unstructured.NestedString(hook.Object, "spec", "serviceAccount")
	if !found {
		return ""
	}
	return serviceAccount
}

// extractHookDeadline gets the deadline from the hook spec
func extractHookDeadline(hook unstructured.Unstructured) string {
	deadline, found, _ := unstructured.NestedInt64(hook.Object, "spec", "deadline")
	if !found || deadline == 0 {
		return ""
	}
	return fmt.Sprintf("%ds", deadline)
}

// extractHookPlaybookStatus gets the playbook status (whether it has content)
func extractHookPlaybookStatus(hook unstructured.Unstructured) string {
	playbook, found, _ := unstructured.NestedString(hook.Object, "spec", "playbook")
	if !found || playbook == "" {
		return "No"
	}
	return "Yes"
}

// extractHookStatus gets the status from the hook
func extractHookStatus(hook unstructured.Unstructured) string {
	// Check for conditions array
	conditions, found, _ := unstructured.NestedSlice(hook.Object, "status", "conditions")
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

// createHookItem creates a standardized hook item for output
func createHookItem(hook unstructured.Unstructured, useUTC bool) map[string]interface{} {
	item := map[string]interface{}{
		"name":           hook.GetName(),
		"namespace":      hook.GetNamespace(),
		"image":          extractHookImage(hook),
		"serviceAccount": extractHookServiceAccount(hook),
		"deadline":       extractHookDeadline(hook),
		"playbook":       extractHookPlaybookStatus(hook),
		"status":         extractHookStatus(hook),
		"created":        output.FormatTimestamp(hook.GetCreationTimestamp().Time, useUTC),
		"object":         hook.Object, // Include the original object
	}

	return item
}

// List lists hooks
func List(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace, outputFormat string, hookName string, useUTC bool) error {
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

	// If hookName is specified, get that specific hook
	if hookName != "" {
		allItems, err = getSpecificHook(ctx, dynamicClient, namespace, hookName, useUTC)
	} else {
		// Get all hooks
		allItems, err = getAllHooks(ctx, dynamicClient, namespace, useUTC)
	}

	// Handle error if no items found
	if err != nil {
		return err
	}

	// Handle output based on format
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(allItems, "No hooks found.")
	case "yaml":
		return output.PrintYAMLWithEmpty(allItems, "No hooks found.")
	default: // table
		return printHookTable(allItems)
	}
}

// getAllHooks retrieves all hooks from the given namespace
func getAllHooks(ctx context.Context, dynamicClient dynamic.Interface, namespace string, useUTC bool) ([]map[string]interface{}, error) {
	hooks, err := dynamicClient.Resource(client.HooksGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list hooks: %v", err)
	}

	allItems := make([]map[string]interface{}, 0, len(hooks.Items))
	for _, hook := range hooks.Items {
		allItems = append(allItems, createHookItem(hook, useUTC))
	}

	return allItems, nil
}

// getSpecificHook retrieves a specific hook by name
func getSpecificHook(ctx context.Context, dynamicClient dynamic.Interface, namespace string, hookName string, useUTC bool) ([]map[string]interface{}, error) {
	hook, err := dynamicClient.Resource(client.HooksGVR).Namespace(namespace).Get(ctx, hookName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get hook '%s': %v", hookName, err)
	}

	allItems := []map[string]interface{}{createHookItem(*hook, useUTC)}
	return allItems, nil
}

// printHookTable prints hooks in table format
func printHookTable(items []map[string]interface{}) error {
	if len(items) == 0 {
		fmt.Println("No hooks found.")
		return nil
	}

	// Create table headers
	headers := []string{"NAME", "IMAGE", "SERVICE ACCOUNT", "DEADLINE", "PLAYBOOK", "STATUS", "CREATED"}

	// Prepare table data
	var data [][]string
	for _, item := range items {
		serviceAccount := fmt.Sprintf("%v", item["serviceAccount"])
		if serviceAccount == "" {
			serviceAccount = "-"
		}
		deadline := fmt.Sprintf("%v", item["deadline"])
		if deadline == "" {
			deadline = "-"
		}

		row := []string{
			fmt.Sprintf("%v", item["name"]),
			fmt.Sprintf("%v", item["image"]),
			serviceAccount,
			deadline,
			fmt.Sprintf("%v", item["playbook"]),
			fmt.Sprintf("%v", item["status"]),
			fmt.Sprintf("%v", item["created"]),
		}
		data = append(data, row)
	}

	// Print the table using TablePrinter
	printer := output.NewTablePrinter()

	// Create headers using Header struct
	var tableHeaders []output.Header
	headerMappings := map[string]string{
		"NAME":            "name",
		"IMAGE":           "image",
		"SERVICE ACCOUNT": "serviceaccount",
		"DEADLINE":        "deadline",
		"PLAYBOOK":        "playbook",
		"STATUS":          "status",
		"CREATED":         "created",
	}

	for _, header := range headers {
		tableHeaders = append(tableHeaders, output.Header{
			DisplayName: header,
			JSONPath:    headerMappings[header],
		})
	}

	printer.WithHeaders(tableHeaders...)

	// Convert data to map format for the table printer
	for _, row := range data {
		item := map[string]interface{}{
			"name":           row[0],
			"image":          row[1],
			"serviceaccount": row[2],
			"deadline":       row[3],
			"playbook":       row[4],
			"status":         row[5],
			"created":        row[6],
		}
		printer.AddItem(item)
	}

	return printer.Print()
}

// GetHookPlaybookContent extracts and decodes the playbook content from a hook
func GetHookPlaybookContent(hook unstructured.Unstructured) (string, error) {
	playbook, found, _ := unstructured.NestedString(hook.Object, "spec", "playbook")
	if !found || playbook == "" {
		return "", nil // No playbook content
	}

	// Decode the base64 content
	decoded, err := base64.StdEncoding.DecodeString(playbook)
	if err != nil {
		return "", fmt.Errorf("failed to decode playbook content: %v", err)
	}

	return string(decoded), nil
}
