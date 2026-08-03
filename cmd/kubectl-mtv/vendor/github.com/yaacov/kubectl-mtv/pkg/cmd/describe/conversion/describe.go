package conversion

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

// Describe prints detailed information about a conversion resource
func Describe(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, useUTC bool, outputFormat string) error {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	conv, err := dynamicClient.Resource(client.ConversionsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get conversion '%s': %v", name, err)
	}

	if outputFormat == "json" {
		return output.PrintJSONWithEmpty([]map[string]interface{}{{"object": conv.Object}}, "")
	}
	if outputFormat == "yaml" {
		return output.PrintYAMLWithEmpty([]map[string]interface{}{{"object": conv.Object}}, "")
	}

	printConversionDetails(conv, useUTC)
	return nil
}

func printConversionDetails(conv *unstructured.Unstructured, useUTC bool) {
	fmt.Printf("Name:       %s\n", conv.GetName())
	fmt.Printf("Namespace:  %s\n", conv.GetNamespace())
	fmt.Printf("Created:    %s\n", output.FormatTimestamp(conv.GetCreationTimestamp().Time, useUTC))
	fmt.Println()

	// Spec
	fmt.Println("Spec:")
	if t, _, _ := unstructured.NestedString(conv.Object, "spec", "type"); t != "" {
		fmt.Printf("  Type:              %s\n", t)
	}
	if vmName, _, _ := unstructured.NestedString(conv.Object, "spec", "vm", "name"); vmName != "" {
		fmt.Printf("  VM:                %s\n", vmName)
	} else if vmID, _, _ := unstructured.NestedString(conv.Object, "spec", "vm", "id"); vmID != "" {
		fmt.Printf("  VM ID:             %s\n", vmID)
	}
	if image, _, _ := unstructured.NestedString(conv.Object, "spec", "image"); image != "" {
		fmt.Printf("  Image:             %s\n", image)
	}
	if ns, _, _ := unstructured.NestedString(conv.Object, "spec", "targetNamespace"); ns != "" {
		fmt.Printf("  Target Namespace:  %s\n", ns)
	}
	if vddk, _, _ := unstructured.NestedString(conv.Object, "spec", "vddkImage"); vddk != "" {
		fmt.Printf("  VDDK Image:        %s\n", vddk)
	}

	// Disks
	disks, _, _ := unstructured.NestedSlice(conv.Object, "spec", "disks")
	if len(disks) > 0 {
		fmt.Printf("  Disks:             %d\n", len(disks))
		for i, d := range disks {
			if dm, ok := d.(map[string]interface{}); ok {
				diskName, _ := dm["name"].(string)
				fmt.Printf("    [%d] %s\n", i, diskName)
			}
		}
	}

	fmt.Println()

	// Status
	fmt.Println("Status:")
	if phase, _, _ := unstructured.NestedString(conv.Object, "status", "phase"); phase != "" {
		fmt.Printf("  Phase:    %s\n", phase)
	} else {
		fmt.Printf("  Phase:    Pending\n")
	}
	if stage, _, _ := unstructured.NestedString(conv.Object, "status", "stage"); stage != "" {
		fmt.Printf("  Stage:    %s\n", stage)
	}
	if msg, _, _ := unstructured.NestedString(conv.Object, "status", "message"); msg != "" {
		fmt.Printf("  Message:  %s\n", msg)
	}

	// Pod reference
	if podName, _, _ := unstructured.NestedString(conv.Object, "status", "pod", "name"); podName != "" {
		podNs, _, _ := unstructured.NestedString(conv.Object, "status", "pod", "namespace")
		if podNs != "" {
			fmt.Printf("  Pod:      %s/%s\n", podNs, podName)
		} else {
			fmt.Printf("  Pod:      %s\n", podName)
		}
	}

	// Timing
	if startTime, _, _ := unstructured.NestedString(conv.Object, "status", "startTime"); startTime != "" {
		fmt.Printf("  Started:  %s\n", startTime)
	}
	if completionTime, _, _ := unstructured.NestedString(conv.Object, "status", "completionTime"); completionTime != "" {
		fmt.Printf("  Completed: %s\n", completionTime)
	}

	// Snapshot status
	snapshot, found, _ := unstructured.NestedMap(conv.Object, "status", "snapshot")
	if found && len(snapshot) > 0 {
		fmt.Println()
		fmt.Println("Snapshot:")
		if moref, ok := snapshot["moref"].(string); ok && moref != "" {
			fmt.Printf("  MoRef:   %s\n", moref)
		}
		if owned, ok := snapshot["owned"].(bool); ok {
			fmt.Printf("  Owned:   %v\n", owned)
		}
	}

	// Inspection result
	inspection, found, _ := unstructured.NestedMap(conv.Object, "status", "inspectionResult")
	if found && len(inspection) > 0 {
		fmt.Println()
		fmt.Println("Inspection Result:")
		if passed, ok := inspection["allChecksPassed"].(bool); ok {
			fmt.Printf("  All Checks Passed: %v\n", passed)
		}
		if osInfo, ok := inspection["osInfo"].(map[string]interface{}); ok {
			var osParts []string
			if name, ok := osInfo["name"].(string); ok && name != "" {
				osParts = append(osParts, name)
			}
			if distro, ok := osInfo["distro"].(string); ok && distro != "" {
				osParts = append(osParts, distro)
			}
			if version, ok := osInfo["version"].(string); ok && version != "" {
				osParts = append(osParts, version)
			}
			if arch, ok := osInfo["arch"].(string); ok && arch != "" {
				osParts = append(osParts, arch)
			}
			if len(osParts) > 0 {
				fmt.Printf("  OS: %s\n", strings.Join(osParts, " "))
			}
		}
		if concerns, ok := inspection["concerns"].([]interface{}); ok && len(concerns) > 0 {
			fmt.Printf("  Concerns: %d\n", len(concerns))
			for _, c := range concerns {
				if cm, ok := c.(map[string]interface{}); ok {
					label, _ := cm["label"].(string)
					category, _ := cm["category"].(string)
					fmt.Printf("    - [%s] %s\n", category, label)
				}
			}
		}
	}

	// Conditions
	conditions, _, _ := unstructured.NestedSlice(conv.Object, "status", "conditions")
	if len(conditions) > 0 {
		fmt.Println()
		fmt.Println("Conditions:")
		for _, c := range conditions {
			if cm, ok := c.(map[string]interface{}); ok {
				condType, _ := cm["type"].(string)
				status, _ := cm["status"].(string)
				reason, _ := cm["reason"].(string)
				message, _ := cm["message"].(string)
				line := fmt.Sprintf("  %s: %s", condType, status)
				if reason != "" {
					line += fmt.Sprintf(" (%s)", reason)
				}
				fmt.Println(line)
				if message != "" {
					fmt.Printf("    %s\n", message)
				}
			}
		}
	}
}
