package mapping

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe describes a network or storage mapping
func Describe(configFlags *genericclioptions.ConfigFlags, mappingType, name, namespace string, useUTC bool) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Select appropriate GVR based on mapping type
	var gvr = client.NetworkMapGVR
	var resourceType = "NETWORK MAPPING"
	if mappingType == "storage" {
		gvr = client.StorageMapGVR
		resourceType = "STORAGE MAPPING"
	}

	// Get the mapping
	mapping, err := c.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get %s mapping: %v", mappingType, err)
	}

	// Print the mapping details
	fmt.Printf("\n%s", output.ColorizedSeparator(105, output.YellowColor))
	fmt.Printf("\n%s\n", output.Cyan(resourceType))

	// Basic Information
	fmt.Printf("%s %s\n", output.Bold("Name:"), output.Yellow(mapping.GetName()))
	fmt.Printf("%s %s\n", output.Bold("Namespace:"), output.Yellow(mapping.GetNamespace()))
	fmt.Printf("%s %s\n", output.Bold("Created:"), output.Yellow(output.FormatTimestamp(mapping.GetCreationTimestamp().Time, useUTC)))

	// Provider Information
	if sourceProvider, found, _ := unstructured.NestedMap(mapping.Object, "spec", "provider", "source"); found {
		if sourceName, ok := sourceProvider["name"].(string); ok {
			fmt.Printf("%s %s\n", output.Bold("Source Provider:"), output.Yellow(sourceName))
		}
	}

	if destProvider, found, _ := unstructured.NestedMap(mapping.Object, "spec", "provider", "destination"); found {
		if destName, ok := destProvider["name"].(string); ok {
			fmt.Printf("%s %s\n", output.Bold("Destination Provider:"), output.Yellow(destName))
		}
	}

	// Owner References
	if len(mapping.GetOwnerReferences()) > 0 {
		fmt.Printf("\n%s\n", output.Cyan("OWNERSHIP"))
		for _, owner := range mapping.GetOwnerReferences() {
			fmt.Printf("%s %s/%s", output.Bold("Owner:"), owner.Kind, owner.Name)
			if owner.Controller != nil && *owner.Controller {
				fmt.Printf(" %s", output.Green("(controller)"))
			}
			fmt.Println()
		}
	}

	// Mapping Specification
	if err := displayMappingSpec(mapping, mappingType); err != nil {
		fmt.Printf("\n%s: %v\n", output.Bold("Mapping Details"), output.Red("Failed to display"))
	}

	// Status Information
	if status, found, _ := unstructured.NestedMap(mapping.Object, "status"); found && status != nil {
		fmt.Printf("\n%s\n", output.Cyan("STATUS"))

		// Conditions
		if conditions, found, _ := unstructured.NestedSlice(mapping.Object, "status", "conditions"); found {
			output.PrintConditions(conditions)
		}

		// Other status fields
		if observedGeneration, found, _ := unstructured.NestedInt64(mapping.Object, "status", "observedGeneration"); found {
			fmt.Printf("%s %d\n", output.Bold("Observed Generation:"), observedGeneration)
		}
	}

	// Annotations
	if annotations := mapping.GetAnnotations(); len(annotations) > 0 {
		fmt.Printf("\n%s\n", output.Cyan("ANNOTATIONS"))
		for key, value := range annotations {
			fmt.Printf("%s: %s\n", output.Bold(key), value)
		}
	}

	// Labels
	if labels := mapping.GetLabels(); len(labels) > 0 {
		fmt.Printf("\n%s\n", output.Cyan("LABELS"))
		for key, value := range labels {
			fmt.Printf("%s: %s\n", output.Bold(key), value)
		}
	}

	fmt.Println() // Add a newline at the end
	return nil
}

// displayMappingSpec displays the mapping specification details with custom formatting
func displayMappingSpec(mapping *unstructured.Unstructured, mappingType string) error {
	// Get the map entries
	mapEntries, found, _ := unstructured.NestedSlice(mapping.Object, "spec", "map")
	if !found || len(mapEntries) == 0 {
		return fmt.Errorf("no mapping entries found")
	}

	fmt.Printf("\n%s\n", output.Cyan("MAPPING ENTRIES"))

	return output.PrintMappingTable(mapEntries, formatMappingEntry)
}

// formatMappingEntry formats a single mapping entry (source or destination) as a string
func formatMappingEntry(entryMap map[string]interface{}, entryType string) string {
	entry, found, _ := unstructured.NestedMap(entryMap, entryType)
	if !found {
		return ""
	}

	var parts []string

	// Common fields that might be present
	if id, ok := entry["id"].(string); ok && id != "" {
		parts = append(parts, fmt.Sprintf("ID: %s", id))
	}

	if name, ok := entry["name"].(string); ok && name != "" {
		parts = append(parts, fmt.Sprintf("Name: %s", name))
	}

	if path, ok := entry["path"].(string); ok && path != "" {
		parts = append(parts, fmt.Sprintf("Path: %s", path))
	}

	// For storage mappings
	if storageClass, ok := entry["storageClass"].(string); ok && storageClass != "" {
		parts = append(parts, fmt.Sprintf("Storage Class: %s", storageClass))
	}

	if accessMode, ok := entry["accessMode"].(string); ok && accessMode != "" {
		parts = append(parts, fmt.Sprintf("Access Mode: %s", accessMode))
	}

	// For network mappings
	if vlan, ok := entry["vlan"].(string); ok && vlan != "" {
		parts = append(parts, fmt.Sprintf("VLAN: %s", vlan))
	}

	if multus, found, _ := unstructured.NestedMap(entry, "multus"); found {
		if networkName, ok := multus["networkName"].(string); ok && networkName != "" {
			parts = append(parts, fmt.Sprintf("Multus Network: %s", networkName))
		}
	}

	// Any other string fields that might be interesting
	for key, value := range entry {
		if strValue, ok := value.(string); ok && strValue != "" {
			// Skip fields we've already handled
			if key != "id" && key != "name" && key != "path" && key != "storageClass" &&
				key != "accessMode" && key != "vlan" && key != "multus" {
				// Capitalize first letter for display
				displayKey := strings.ToUpper(key[:1]) + key[1:]
				parts = append(parts, fmt.Sprintf("%s: %s", displayKey, strValue))
			}
		}
	}

	// Join all parts with newlines for multi-line cell display
	return strings.Join(parts, "\n")
}
