package host

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe describes a migration host.
func Describe(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, useUTC bool, insecureSkipTLS bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	host, err := c.Resource(client.HostsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get host: %v", err)
	}

	b := describe.NewBuilder("MIGRATION HOST")

	// Basic information
	b.Field("Name", host.GetName())
	b.Field("Namespace", host.GetNamespace())
	b.Field("Created", output.FormatTimestamp(host.GetCreationTimestamp().Time, useUTC))

	if hostID, found, _ := unstructured.NestedString(host.Object, "spec", "id"); found {
		b.Field("Host ID", hostID)
	}
	if hostName, found, _ := unstructured.NestedString(host.Object, "spec", "name"); found {
		b.Field("Host Name", hostName)
	}
	if ip, found, _ := unstructured.NestedString(host.Object, "spec", "ipAddress"); found {
		b.Field("IP Address", ip)
	}

	if providerMap, found, _ := unstructured.NestedMap(host.Object, "spec", "provider"); found {
		if pname, ok := providerMap["name"].(string); ok {
			b.Field("Provider", pname)
		}
	}
	if secretMap, found, _ := unstructured.NestedMap(host.Object, "spec", "secret"); found {
		if sname, ok := secretMap["name"].(string); ok {
			b.Field("Secret", sname)
		}
	}

	// Ownership
	if owners := host.GetOwnerReferences(); len(owners) > 0 {
		b.Section("OWNERSHIP")
		for _, owner := range owners {
			val := owner.Kind + "/" + owner.Name
			if owner.Controller != nil && *owner.Controller {
				val += " (controller)"
			}
			b.Field("Owner", val)
		}
	}

	// Network adapters from inventory
	buildNetworkAdapters(ctx, configFlags, b, host, namespace, insecureSkipTLS)

	// Status / conditions
	if conditions, found, _ := unstructured.NestedSlice(host.Object, "status", "conditions"); found && len(conditions) > 0 {
		b.Section("STATUS")
		addConditionsTable(b, conditions)
	}

	if gen, found, _ := unstructured.NestedInt64(host.Object, "status", "observedGeneration"); found {
		b.Field("Observed Generation", fmt.Sprintf("%d", gen))
	}

	// Annotations & labels
	addAnnotationsAndLabels(b, host)

	return describe.Print(b.Build(), outputFormat)
}

func buildNetworkAdapters(ctx context.Context, configFlags *genericclioptions.ConfigFlags, b *describe.Builder, host *unstructured.Unstructured, namespace string, insecureSkipTLS bool) {
	hostID, found, _ := unstructured.NestedString(host.Object, "spec", "id")
	if !found || hostID == "" {
		return
	}

	providerMap, found, _ := unstructured.NestedMap(host.Object, "spec", "provider")
	if !found {
		return
	}
	providerName, ok := providerMap["name"].(string)
	if !ok || providerName == "" {
		return
	}

	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return
	}

	inventoryURL := client.DiscoverInventoryURL(ctx, configFlags, namespace)
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, insecureSkipTLS)

	providerType, err := providerClient.GetProviderType()
	if err != nil || (providerType != "ovirt" && providerType != "vsphere") {
		return
	}

	hostData, err := providerClient.GetHost(ctx, hostID, 4)
	if err != nil {
		return
	}

	hostMap, ok := hostData.(map[string]interface{})
	if !ok {
		return
	}

	adapters, found, _ := unstructured.NestedSlice(hostMap, "networkAdapters")
	if !found || len(adapters) == 0 {
		return
	}

	b.Section("NETWORK ADAPTERS")

	headers := []describe.TableColumn{
		{Display: "NAME", Key: "name"},
		{Display: "IP ADDRESS", Key: "ip"},
		{Display: "SUBNET MASK", Key: "subnet"},
		{Display: "MTU", Key: "mtu"},
		{Display: "LINK SPEED", Key: "speed"},
	}

	rows := make([]map[string]string, 0, len(adapters))
	for _, adapter := range adapters {
		adapterMap, ok := adapter.(map[string]interface{})
		if !ok {
			continue
		}
		row := map[string]string{}
		if n, ok := adapterMap["name"].(string); ok {
			row["name"] = n
		}
		if ip, ok := adapterMap["ipAddress"].(string); ok {
			row["ip"] = ip
		}
		if sm, ok := adapterMap["subnetMask"].(string); ok {
			row["subnet"] = sm
		}
		if mtu, ok := adapterMap["mtu"].(float64); ok {
			row["mtu"] = fmt.Sprintf("%.0f", mtu)
		}
		if speed, ok := adapterMap["linkSpeed"].(float64); ok {
			row["speed"] = fmt.Sprintf("%.0f Mbps", speed)
		}
		rows = append(rows, row)
	}

	b.Table(headers, rows)
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
