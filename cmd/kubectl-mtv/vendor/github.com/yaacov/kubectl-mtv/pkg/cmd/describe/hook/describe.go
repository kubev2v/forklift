package hook

import (
	"context"
	"encoding/base64"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe describes a migration hook.
func Describe(configFlags *genericclioptions.ConfigFlags, name, namespace string, useUTC bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	hook, err := c.Resource(client.HooksGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get hook: %v", err)
	}

	b := describe.NewBuilder("MIGRATION HOOK")

	// Basic information
	b.Field("Name", hook.GetName())
	b.Field("Namespace", hook.GetNamespace())
	b.Field("Created", output.FormatTimestamp(hook.GetCreationTimestamp().Time, useUTC))

	if image, found, _ := unstructured.NestedString(hook.Object, "spec", "image"); found {
		b.Field("Image", image)
	}

	if sa, found, _ := unstructured.NestedString(hook.Object, "spec", "serviceAccount"); found && sa != "" {
		b.Field("Service Account", sa)
	} else {
		b.Field("Service Account", "(default)")
	}

	if deadline, found, _ := unstructured.NestedInt64(hook.Object, "spec", "deadline"); found && deadline > 0 {
		b.Field("Deadline", fmt.Sprintf("%d seconds", deadline))
	} else {
		b.Field("Deadline", "(unlimited)")
	}

	// Playbook
	playbook, playbookFound, _ := unstructured.NestedString(hook.Object, "spec", "playbook")
	if playbookFound && playbook != "" {
		b.FieldC("Playbook", "Yes", output.Green)

		if decoded, err := base64.StdEncoding.DecodeString(playbook); err == nil {
			b.Section("PLAYBOOK CONTENT")
			b.Text("", string(decoded), "yaml")
		} else {
			b.FieldC("Playbook Decoding", "Failed - invalid base64", output.Red)
		}
	} else {
		b.Field("Playbook", "No")
	}

	// Status / conditions
	if conditions, found, _ := unstructured.NestedSlice(hook.Object, "status", "conditions"); found && len(conditions) > 0 {
		b.Section("STATUS")
		addConditionsTable(b, conditions)
	}

	if gen, found, _ := unstructured.NestedInt64(hook.Object, "status", "observedGeneration"); found {
		b.Field("Observed Generation", fmt.Sprintf("%d", gen))
	}

	// Ownership
	if owners := hook.GetOwnerReferences(); len(owners) > 0 {
		b.Section("OWNERSHIP")
		for _, owner := range owners {
			val := owner.Kind + "/" + owner.Name
			if owner.Controller != nil && *owner.Controller {
				val += " (controller)"
			}
			b.Field("Owner", val)
		}
	}

	// Annotations & labels
	addAnnotationsAndLabels(b, hook)

	// Usage example
	b.Section("USAGE")
	yamlExample := fmt.Sprintf(`spec:
  vms:
    - id: <vm_id>
      hooks:
        - hook:
            namespace: %s
            name: %s
          step: PreHook  # or PostHook`, hook.GetNamespace(), hook.GetName())
	b.Text("", yamlExample, "yaml")
	b.Field("Note", "For a PreHook to run on a VM, the VM must be started and available via SSH.")

	return describe.Print(b.Build(), outputFormat)
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
