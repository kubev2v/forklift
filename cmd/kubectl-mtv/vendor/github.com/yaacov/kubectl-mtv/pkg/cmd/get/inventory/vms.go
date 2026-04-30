package inventory

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	planv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// countConcernsByCategory counts VM concerns by their category
func countConcernsByCategory(vm map[string]interface{}) map[string]int {
	counts := map[string]int{
		"Critical":    0,
		"Warning":     0,
		"Information": 0,
	}

	concerns, exists := vm["concerns"]
	if !exists {
		return counts
	}

	concernsArray, ok := concerns.([]interface{})
	if !ok {
		return counts
	}

	for _, c := range concernsArray {
		concern, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		category, ok := concern["category"].(string)
		if ok {
			counts[category]++
		}
	}

	return counts
}

// calculateTotalDiskCapacity returns the total disk capacity in GB
func calculateTotalDiskCapacity(vm map[string]interface{}) float64 {
	disks, exists := vm["disks"]
	if !exists {
		return 0
	}

	disksArray, ok := disks.([]interface{})
	if !ok {
		return 0
	}

	var totalCapacity float64
	for _, d := range disksArray {
		disk, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		if capacity, ok := disk["capacity"].(float64); ok {
			totalCapacity += capacity
		}
	}

	// Convert to GB (from bytes)
	return totalCapacity / (1024 * 1024 * 1024)
}

// augmentVMInfo adds computed fields to VM data for display purposes.
func augmentVMInfo(vm map[string]interface{}) {
	concernCounts := countConcernsByCategory(vm)
	vm["criticalConcerns"] = concernCounts["Critical"]
	vm["warningConcerns"] = concernCounts["Warning"]
	vm["infoConcerns"] = concernCounts["Information"]

	vm["concernsHuman"] = fmt.Sprintf("%d/%d/%d",
		concernCounts["Critical"],
		concernCounts["Warning"],
		concernCounts["Information"])

	if concernCounts["Critical"] > 0 {
		vm["concernsHuman"] = vm["concernsHuman"].(string) + " (*)"
	}

	if memoryMB, exists := vm["memoryMB"]; exists {
		if memVal, ok := memoryMB.(float64); ok {
			vm["memoryGB"] = fmt.Sprintf("%.1f GB", memVal/1024)
		}
	}

	totalDiskCapacityGB := calculateTotalDiskCapacity(vm)
	vm["diskCapacity"] = fmt.Sprintf("%.1f GB", totalDiskCapacityGB)

	if storageUsed, exists := vm["storageUsed"]; exists {
		if storageVal, ok := storageUsed.(float64); ok {
			storageUsedGB := storageVal / (1024 * 1024 * 1024)
			vm["storageUsedGB"] = fmt.Sprintf("%.1f GB", storageUsedGB)
		}
	}

	augmentFromInstance(vm)

	vm["powerStateHuman"] = humanizePowerState(vm)
}

// humanizePowerState derives a human-readable power state from a VM map.
// It checks top-level powerState (vSphere, oVirt), then falls back to
// object.status.printableStatus, object.status.phase, and
// instance.status.phase (OpenShift/KubeVirt).
func humanizePowerState(vm map[string]interface{}) string {
	if ps, ok := vm["powerState"].(string); ok && ps != "" {
		if strings.Contains(strings.ToLower(ps), "on") || strings.Contains(strings.ToLower(ps), "up") {
			return "On"
		}
		return "Off"
	}

	if ps, found, _ := unstructured.NestedString(vm, "object", "status", "printableStatus"); found && ps != "" {
		lower := strings.ToLower(ps)
		if strings.Contains(lower, "running") {
			return "On"
		}
		if strings.Contains(lower, "stopped") || strings.Contains(lower, "off") || strings.Contains(lower, "halted") {
			return "Off"
		}
		return ps
	}

	for _, prefix := range []string{"object", "instance"} {
		if phase, found, _ := unstructured.NestedString(vm, prefix, "status", "phase"); found && phase != "" {
			lower := strings.ToLower(phase)
			if strings.Contains(lower, "running") {
				return "On"
			}
			if strings.Contains(lower, "stopped") || strings.Contains(lower, "off") {
				return "Off"
			}
			return phase
		}
	}

	return ""
}

// augmentFromInstance extracts runtime info from the optional VirtualMachineInstance
// data present on OpenShift/KubeVirt VMs. It only fills in fields that are not
// already populated by the provider inventory.
func augmentFromInstance(vm map[string]interface{}) {
	instance, ok := vm["instance"].(map[string]interface{})
	if !ok {
		return
	}

	// CPU count from instance spec (cores * sockets * threads)
	if _, exists := vm["cpuCount"]; !exists {
		cores, cFound, _ := unstructured.NestedFloat64(instance, "spec", "domain", "cpu", "cores")
		sockets, sFound, _ := unstructured.NestedFloat64(instance, "spec", "domain", "cpu", "sockets")
		threads, tFound, _ := unstructured.NestedFloat64(instance, "spec", "domain", "cpu", "threads")

		if cFound && cores > 0 {
			if !sFound || sockets < 1 {
				sockets = 1
			}
			if !tFound || threads < 1 {
				threads = 1
			}
			vm["cpuCount"] = int64(cores * sockets * threads)
		}
	}

	// Memory from instance spec or status
	if _, exists := vm["memoryGB"]; !exists {
		memStr := ""
		if s, found, _ := unstructured.NestedString(instance, "status", "memory", "guestCurrent"); found && s != "" {
			memStr = s
		} else if s, found, _ := unstructured.NestedString(instance, "spec", "domain", "memory", "guest"); found && s != "" {
			memStr = s
		}
		if memStr != "" {
			if q, err := resource.ParseQuantity(memStr); err == nil {
				memGB := float64(q.Value()) / (1024 * 1024 * 1024)
				vm["memoryGB"] = fmt.Sprintf("%.1f GB", memGB)
			}
		}
	}

	// Guest OS from instance guest agent info
	if _, exists := vm["guestId"]; !exists {
		if name, found, _ := unstructured.NestedString(instance, "status", "guestOSInfo", "prettyName"); found && name != "" {
			vm["guestId"] = name
		} else if id, found, _ := unstructured.NestedString(instance, "status", "guestOSInfo", "id"); found && id != "" {
			vm["guestId"] = id
		}
	}
}

// FetchVMsByQuery fetches VMs from inventory based on a query string and returns them as plan VM structs
func FetchVMsByQuery(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, query string) ([]planv1beta1.VM, error) {
	return FetchVMsByQueryWithInsecure(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, query, false)
}

// FetchVMsByQueryWithInsecure fetches VMs from inventory based on a query string and returns them as plan VM structs with optional insecure TLS skip verification
func FetchVMsByQueryWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, query string, insecureSkipTLS bool) ([]planv1beta1.VM, error) {
	// Validate inputs early
	if providerName == "" {
		return nil, fmt.Errorf("provider name cannot be empty")
	}
	if query == "" {
		return nil, fmt.Errorf("query string cannot be empty")
	}

	// Parse and validate query syntax BEFORE fetching inventory (fail fast)
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return nil, fmt.Errorf("invalid query string: %v", err)
	}

	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return nil, err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify VM support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	// Verify provider supports VM inventory before fetching
	switch providerType {
	case "ovirt", "vsphere", "openstack", "ova", "openshift", "ec2", "hyperv":
		// Provider supports VMs, continue
	default:
		return nil, fmt.Errorf("provider type '%s' does not support VM inventory", providerType)
	}

	// Fetch VM inventory from the provider (expensive operation)
	data, err := providerClient.GetVMs(ctx, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VM inventory: %v", err)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VM inventory")
	}

	// Convert to expected format
	vms := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if vm, ok := item.(map[string]interface{}); ok {
			// Add provider name to each VM
			vm["provider"] = providerName
			vms = append(vms, vm)
		}
	}

	// Apply query options (sorting, filtering, limiting)
	vms, err = querypkg.ApplyQuery(vms, queryOpts)
	if err != nil {
		return nil, fmt.Errorf("error applying query: %v", err)
	}

	// Convert inventory VMs to plan VM structs
	planVMs := make([]planv1beta1.VM, 0, len(vms))
	for _, vm := range vms {
		vmName, ok := vm["name"].(string)
		if !ok {
			continue
		}

		planVM := planv1beta1.VM{}
		planVM.Name = vmName

		// Add ID if available
		if vmID, ok := vm["id"].(string); ok {
			planVM.ID = vmID
		}

		planVMs = append(planVMs, planVM)
	}

	return planVMs, nil
}

// ListVMsWithInsecure queries the provider's VM inventory and displays the results with optional insecure TLS skip verification.
func ListVMsWithInsecure(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, watchMode bool, insecureSkipTLS bool) error {
	sq := watch.NewSafeQuery(query)

	return watch.WrapWithWatchAndQuery(watchMode, outputFormat, func() error {
		return listVMsOnce(ctx, kubeConfigFlags, providerName, namespace, inventoryURL, outputFormat, sq.Get(), insecureSkipTLS)
	}, watch.DefaultInterval, sq.Set, query)
}

func listVMsOnce(ctx context.Context, kubeConfigFlags *genericclioptions.ConfigFlags, providerName, namespace string, inventoryURL string, outputFormat string, query string, insecureSkipTLS bool) error {
	// Get the provider object
	provider, err := GetProviderByName(ctx, kubeConfigFlags, providerName, namespace)
	if err != nil {
		return err
	}

	// Create a new provider client
	providerClient := NewProviderClientWithInsecure(kubeConfigFlags, provider, inventoryURL, insecureSkipTLS)

	// Get provider type to verify VM support
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return fmt.Errorf("failed to get provider type: %v", err)
	}

	// Fetch VM inventory from the provider based on provider type
	var data interface{}
	switch providerType {
	case "ovirt", "vsphere", "openstack", "ova", "openshift", "ec2", "hyperv":
		data, err = providerClient.GetVMs(ctx, 4)
	default:
		return fmt.Errorf("provider type '%s' does not support VM inventory", providerType)
	}

	// Error handling
	if err != nil {
		return fmt.Errorf("failed to fetch VM inventory: %v", err)
	}

	// Extract objects from EC2 envelope
	if providerType == "ec2" {
		data = ExtractEC2Objects(data)
	}

	// Verify data is an array
	dataArray, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for VM inventory")
	}

	// Convert to expected format
	vms := make([]map[string]interface{}, 0, len(dataArray))
	for _, item := range dataArray {
		if vm, ok := item.(map[string]interface{}); ok {
			vm["provider"] = providerName

			if providerType != "ec2" {
				augmentVMInfo(vm)
			}

			vms = append(vms, vm)
		}
	}

	// Parse and apply query options
	queryOpts, err := querypkg.ParseQueryString(query)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	// Apply query options (sorting, filtering, limiting)
	vms, err = querypkg.ApplyQuery(vms, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Format validation
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" && outputFormat != "markdown" && outputFormat != "planvms" {
		return fmt.Errorf("unsupported output format: %s. Supported formats: table, json, yaml, markdown, planvms", outputFormat)
	}

	// Handle different output formats
	emptyMessage := fmt.Sprintf("No VMs found for provider %s", providerName)
	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(vms, emptyMessage)
	case "yaml":
		return output.PrintYAMLWithEmpty(vms, emptyMessage)
	case "markdown":
		return printVMsMarkdown(vms, queryOpts, providerType, emptyMessage)
	case "planvms":
		// Convert inventory VMs to plan VM structs
		planVMs := make([]planv1beta1.VM, 0, len(vms))
		for _, vm := range vms {
			vmName, ok := vm["name"].(string)
			if !ok {
				continue
			}

			planVM := planv1beta1.VM{}
			planVM.Name = vmName

			// Add ID if available
			if vmID, ok := vm["id"].(string); ok {
				planVM.ID = vmID
			}

			planVMs = append(planVMs, planVM)
		}

		// Marshal to YAML
		yamlData, err := yaml.Marshal(planVMs)
		if err != nil {
			return fmt.Errorf("failed to marshal plan VMs to YAML: %v", err)
		}

		// Print the YAML to stdout
		fmt.Println(string(yamlData))
		return nil
	default:
		return output.PrintTableWithQuery(vms, vmColumns(providerType), queryOpts, emptyMessage)
	}
}

func printVMsMarkdown(vms []map[string]interface{}, queryOpts *querypkg.QueryOptions, providerType, emptyMessage string) error {
	return output.PrintMarkdownWithQuery(vms, vmColumns(providerType), queryOpts, emptyMessage)
}

// vmColumns returns the default table columns for VM listings based on provider type.
func vmColumns(providerType string) []output.Column {
	if providerType == "ec2" {
		return []output.Column{
			{Title: "NAME", Key: "name"},
			{Title: "TYPE", Key: "InstanceType"},
			{Title: "STATE", Key: "State.Name", ColorFunc: output.ColorizeStatus},
			{Title: "PLATFORM", Key: "PlatformDetails"},
			{Title: "AZ", Key: "Placement.AvailabilityZone"},
			{Title: "PUBLIC-IP", Key: "PublicIpAddress"},
			{Title: "PRIVATE-IP", Key: "PrivateIpAddress"},
		}
	}
	return []output.Column{
		{Title: "NAME", Key: "name"},
		{Title: "ID", Key: "id"},
		{Title: "POWER", Key: "powerStateHuman", ColorFunc: output.ColorizePowerState},
		{Title: "CPU", Key: "cpuCount"},
		{Title: "MEMORY", Key: "memoryGB"},
		{Title: "DISK USAGE", Key: "storageUsedGB"},
		{Title: "GUEST OS", Key: "guestId"},
		{Title: "CONCERNS (C/W/I)", Key: "concernsHuman", ColorFunc: output.ColorizeConcerns},
	}
}
