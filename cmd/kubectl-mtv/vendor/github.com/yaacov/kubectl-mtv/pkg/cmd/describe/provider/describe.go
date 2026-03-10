package provider

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/providerutil"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe displays detailed information about a migration provider.
func Describe(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, useUTC bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	provider, err := c.Resource(client.ProvidersGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get provider: %v", err)
	}

	b := describe.NewBuilder("MIGRATION PROVIDER")

	b.Field("Name", provider.GetName())
	b.Field("Namespace", provider.GetNamespace())
	b.Field("Created", output.FormatTimestamp(provider.GetCreationTimestamp().Time, useUTC))

	providerType, _, _ := unstructured.NestedString(provider.Object, "spec", "type")
	b.Field("Type", providerType)

	if url, found, _ := unstructured.NestedString(provider.Object, "spec", "url"); found && url != "" {
		b.Field("URL", url)
	}

	if phase, found, _ := unstructured.NestedString(provider.Object, "status", "phase"); found {
		b.FieldC("Phase", phase, output.ColorizeStatus)
	}

	// Condition summary
	condStatuses := providerutil.ExtractProviderConditionStatuses(provider.Object)
	b.FieldC("Connected", condStatuses.ConnectionStatus, output.ColorizeConditionStatus)
	b.FieldC("Validated", condStatuses.ValidationStatus, output.ColorizeConditionStatus)
	b.FieldC("Inventory", condStatuses.InventoryStatus, output.ColorizeConditionStatus)
	b.FieldC("Ready", condStatuses.ReadyStatus, output.ColorizeConditionStatus)

	// Provider-type specific settings
	buildSettingsSection(b, provider, providerType)

	// Secret reference
	if secretMap, found, _ := unstructured.NestedMap(provider.Object, "spec", "secret"); found {
		if sname, ok := secretMap["name"].(string); ok {
			b.Section("SECRET")
			b.Field("Name", sname)
			if sns, ok := secretMap["namespace"].(string); ok && sns != "" {
				b.Field("Namespace", sns)
			}
		}
	}

	// Status conditions detail
	if conditions, found, _ := unstructured.NestedSlice(provider.Object, "status", "conditions"); found && len(conditions) > 0 {
		b.Section("CONDITIONS")
		addConditionsTable(b, conditions, useUTC)
	}

	if gen, found, _ := unstructured.NestedInt64(provider.Object, "status", "observedGeneration"); found {
		b.Field("Observed Generation", fmt.Sprintf("%d", gen))
	}

	// Ownership
	if owners := provider.GetOwnerReferences(); len(owners) > 0 {
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
	addAnnotationsAndLabels(b, provider)

	return describe.Print(b.Build(), outputFormat)
}

func buildSettingsSection(b *describe.Builder, provider *unstructured.Unstructured, providerType string) {
	b.Section("SETTINGS")

	if sdkEndpoint, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "sdkEndpoint"); found && sdkEndpoint != "" {
		b.Field("SDK Endpoint", sdkEndpoint)
	}

	if vddkImage, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "vddkInitImage"); found && vddkImage != "" {
		b.Field("VDDK Init Image", vddkImage)
	}

	if domain, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "domainName"); found && domain != "" {
		b.Field("Domain Name", domain)
	}

	if project, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "projectName"); found && project != "" {
		b.Field("Project Name", project)
	}

	if region, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "regionName"); found && region != "" {
		b.Field("Region Name", region)
	}

	switch providerType {
	case "vsphere":
		addVSphereSettings(b, provider)
	case "ec2":
		addEC2Settings(b, provider)
	}
}

func addVSphereSettings(b *describe.Builder, provider *unstructured.Unstructured) {
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "esxiCloneMethod"); found && val != "" {
		b.Field("ESXi Clone Method", val)
	}
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "useVddkAioOptimization"); found && val != "" {
		b.Field("VDDK AIO Optimization", val)
	}
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "vddkBufSizeIn64K"); found && val != "" {
		b.Field("VDDK Buffer Size (64K)", val)
	}
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "vddkBufCount"); found && val != "" {
		b.Field("VDDK Buffer Count", val)
	}
}

func addEC2Settings(b *describe.Builder, provider *unstructured.Unstructured) {
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "ec2Region"); found && val != "" {
		b.Field("EC2 Region", val)
	}
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "targetRegion"); found && val != "" {
		b.Field("Target Region", val)
	}
	if val, found, _ := unstructured.NestedString(provider.Object, "spec", "settings", "targetAZ"); found && val != "" {
		b.Field("Target AZ", val)
	}
}

func addConditionsTable(b *describe.Builder, conditions []interface{}, useUTC bool) {
	headers := []describe.TableColumn{
		{Display: "TYPE", Key: "type"},
		{Display: "STATUS", Key: "status", ColorFunc: output.ColorizeConditionStatus},
		{Display: "CATEGORY", Key: "category", ColorFunc: output.ColorizeCategory},
		{Display: "MESSAGE", Key: "message"},
		{Display: "LAST TRANSITION", Key: "lastTransition"},
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
		lastTransition, _ := condMap["lastTransitionTime"].(string)
		if lastTransition != "" {
			lastTransition = output.FormatTime(lastTransition, useUTC)
		}

		rows = append(rows, map[string]string{
			"type":           condType,
			"status":         condStatus,
			"category":       category,
			"message":        message,
			"lastTransition": lastTransition,
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
