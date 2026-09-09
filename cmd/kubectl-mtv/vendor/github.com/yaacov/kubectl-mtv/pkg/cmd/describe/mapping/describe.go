package mapping

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe describes a network or storage mapping.
func Describe(configFlags *genericclioptions.ConfigFlags, mappingType, name, namespace string, useUTC bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	gvr := client.NetworkMapGVR
	resourceTitle := "NETWORK MAPPING"
	if mappingType == "storage" {
		gvr = client.StorageMapGVR
		resourceTitle = "STORAGE MAPPING"
	}

	m, err := c.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get %s mapping: %v", mappingType, err)
	}

	b := describe.NewBuilder(resourceTitle)

	// Basic information
	b.Field("Name", m.GetName())
	b.Field("Namespace", m.GetNamespace())
	b.Field("Created", output.FormatTimestamp(m.GetCreationTimestamp().Time, useUTC))

	if srcProvider, found, _ := unstructured.NestedMap(m.Object, "spec", "provider", "source"); found {
		if sname, ok := srcProvider["name"].(string); ok {
			b.Field("Source Provider", sname)
		}
	}
	if dstProvider, found, _ := unstructured.NestedMap(m.Object, "spec", "provider", "destination"); found {
		if dname, ok := dstProvider["name"].(string); ok {
			b.Field("Destination Provider", dname)
		}
	}

	// Ownership
	if owners := m.GetOwnerReferences(); len(owners) > 0 {
		b.Section("OWNERSHIP")
		for _, owner := range owners {
			val := owner.Kind + "/" + owner.Name
			if owner.Controller != nil && *owner.Controller {
				val += " (controller)"
			}
			b.Field("Owner", val)
		}
	}

	// Mapping entries
	if mapEntries, found, _ := unstructured.NestedSlice(m.Object, "spec", "map"); found && len(mapEntries) > 0 {
		b.Section("MAPPING ENTRIES")

		headers := []describe.TableColumn{
			{Display: "SOURCE", Key: "source"},
			{Display: "DESTINATION", Key: "destination"},
		}

		rows := make([]map[string]string, 0, len(mapEntries))
		for _, entry := range mapEntries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			rows = append(rows, map[string]string{
				"source":      formatMappingEntry(entryMap, "source"),
				"destination": formatMappingEntry(entryMap, "destination"),
			})
		}
		b.Table(headers, rows)
	}

	// Status / conditions
	if conditions, found, _ := unstructured.NestedSlice(m.Object, "status", "conditions"); found && len(conditions) > 0 {
		b.Section("STATUS")
		addConditionsTable(b, conditions)
	}

	if gen, found, _ := unstructured.NestedInt64(m.Object, "status", "observedGeneration"); found {
		b.Field("Observed Generation", fmt.Sprintf("%d", gen))
	}

	// Annotations & labels
	addAnnotationsAndLabels(b, m)

	return describe.Print(b.Build(), outputFormat)
}

func formatMappingEntry(entryMap map[string]interface{}, entryType string) string {
	entry, found, _ := unstructured.NestedMap(entryMap, entryType)
	if !found {
		return ""
	}

	var parts []string

	if id, ok := entry["id"].(string); ok && id != "" {
		parts = append(parts, "ID: "+id)
	}
	if name, ok := entry["name"].(string); ok && name != "" {
		parts = append(parts, "Name: "+name)
	}
	if path, ok := entry["path"].(string); ok && path != "" {
		parts = append(parts, "Path: "+path)
	}
	if sc, ok := entry["storageClass"].(string); ok && sc != "" {
		parts = append(parts, "Storage Class: "+sc)
	}
	if am, ok := entry["accessMode"].(string); ok && am != "" {
		parts = append(parts, "Access Mode: "+am)
	}
	if vlan, ok := entry["vlan"].(string); ok && vlan != "" {
		parts = append(parts, "VLAN: "+vlan)
	}
	if multus, found, _ := unstructured.NestedMap(entry, "multus"); found {
		if nn, ok := multus["networkName"].(string); ok && nn != "" {
			parts = append(parts, "Multus Network: "+nn)
		}
	}
	for key, value := range entry {
		if strValue, ok := value.(string); ok && strValue != "" {
			if key != "id" && key != "name" && key != "path" && key != "storageClass" &&
				key != "accessMode" && key != "vlan" && key != "multus" {
				displayKey := strings.ToUpper(key[:1]) + key[1:]
				parts = append(parts, displayKey+": "+strValue)
			}
		}
	}

	return strings.Join(parts, ", ")
}

func addConditionsTable(b *describe.Builder, conditions []interface{}) {
	headers := []describe.TableColumn{
		{Display: "TYPE", Key: "type"},
		{Display: "STATUS", Key: "status", ColorFunc: output.ColorizeConditionStatus},
		{Display: "CATEGORY", Key: "category", ColorFunc: output.ColorizeCategory},
		{Display: "MESSAGE", Key: "message"},
	}

	rows := make([]map[string]string, 0, len(conditions))
	for _, c := range conditions {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)
		category, _ := condMap["category"].(string)
		message, _ := condMap["message"].(string)
		rows = append(rows, map[string]string{
			"type":     condType,
			"status":   condStatus,
			"category": category,
			"message":  message,
		})
	}

	b.Table(headers, rows)
}

func addAnnotationsAndLabels(b *describe.Builder, obj *unstructured.Unstructured) {
	if annotations := obj.GetAnnotations(); len(annotations) > 0 {
		b.Section("ANNOTATIONS")
		for key, value := range annotations {
			b.Field(key, value)
		}
	}

	if labels := obj.GetLabels(); len(labels) > 0 {
		b.Section("LABELS")
		for key, value := range labels {
			b.Field(key, value)
		}
	}
}
