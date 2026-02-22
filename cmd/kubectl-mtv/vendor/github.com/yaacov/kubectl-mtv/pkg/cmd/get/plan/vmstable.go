package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	querypkg "github.com/yaacov/kubectl-mtv/pkg/util/query"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// vmTableHeaders defines the default columns for the VMs table.
var vmTableHeaders = []output.Header{
	{DisplayName: "VM", JSONPath: "vm"},
	{DisplayName: "SOURCE STATUS", JSONPath: "sourceStatus", ColorFunc: output.ColorizePowerState},
	{DisplayName: "SOURCE IP", JSONPath: "sourceIP"},
	{DisplayName: "TARGET", JSONPath: "target"},
	{DisplayName: "TARGET IP", JSONPath: "targetIP"},
	{DisplayName: "TARGET STATUS", JSONPath: "targetStatus", ColorFunc: output.ColorizePowerState},
	{DisplayName: "PLAN", JSONPath: "plan", ColorFunc: colorizePlanName},
	{DisplayName: "PLAN STATUS", JSONPath: "planStatus", ColorFunc: output.ColorizeStatus},
	{DisplayName: "PROGRESS", JSONPath: "progress"},
}

// colorizePlanName returns a red-colored string when the plan is not ready.
func colorizePlanName(s string) string {
	if strings.Contains(s, "[not ready]") {
		return output.Red(s)
	}
	return s
}

// inventoryCacheEntry holds cached inventory data for a provider.
type inventoryCacheEntry struct {
	vms map[string]map[string]interface{}
	err error
}

// ListVMsTable lists all VMs across plans in a flat table with inventory details.
func ListVMsTable(
	ctx context.Context,
	configFlags *genericclioptions.ConfigFlags,
	planName, namespace, inventoryURL string,
	insecureSkipTLS bool,
	outputFormat, queryStr string,
	watchMode bool,
) error {
	sq := watch.NewSafeQuery(queryStr)

	return watch.WrapWithWatchAndQuery(watchMode, outputFormat, func() error {
		return listVMsTableOnce(ctx, configFlags, planName, namespace, inventoryURL, insecureSkipTLS, outputFormat, sq.Get())
	}, watch.DefaultInterval, sq.Set, queryStr)
}

func listVMsTableOnce(
	ctx context.Context,
	configFlags *genericclioptions.ConfigFlags,
	planName, namespace, inventoryURL string,
	insecureSkipTLS bool,
	outputFormat, queryStr string,
) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Fetch plans
	var plans *unstructured.UnstructuredList
	if planName != "" {
		plans, err = getSpecificPlan(ctx, c, namespace, planName)
		if err != nil {
			return fmt.Errorf("failed to get plan: %v", err)
		}
	} else {
		plans, err = getPlans(ctx, c, namespace)
		if err != nil {
			return fmt.Errorf("failed to list plans: %v", err)
		}
	}

	// Discover inventory URL if not provided
	if inventoryURL == "" {
		inventoryURL = client.DiscoverInventoryURL(ctx, configFlags, namespace)
	}

	// Caches keyed by "namespace/providerName" to avoid redundant API calls
	sourceCache := map[string]*inventoryCacheEntry{}
	targetCache := map[string]*inventoryCacheEntry{}

	// Build flat rows
	items := []map[string]interface{}{}

	for i := range plans.Items {
		p := &plans.Items[i]
		planRows, err := buildPlanVMRows(ctx, configFlags, c, p, namespace, inventoryURL, insecureSkipTLS, sourceCache, targetCache)
		if err != nil {
			klog.V(1).Infof("Warning: failed to build VM rows for plan %s: %v", p.GetName(), err)
			continue
		}
		items = append(items, planRows...)
	}

	// Parse and apply query
	queryOpts, err := querypkg.ParseQueryString(queryStr)
	if err != nil {
		return fmt.Errorf("invalid query string: %v", err)
	}

	items, err = querypkg.ApplyQuery(items, queryOpts)
	if err != nil {
		return fmt.Errorf("error applying query: %v", err)
	}

	// Output
	outputFormat = strings.ToLower(outputFormat)
	emptyMsg := "No VMs found"
	if planName != "" {
		emptyMsg = fmt.Sprintf("No VMs found in plan %s", planName)
	}

	switch outputFormat {
	case "json":
		return output.PrintJSONWithEmpty(items, emptyMsg)
	case "yaml":
		return output.PrintYAMLWithEmpty(items, emptyMsg)
	default:
		return output.PrintTableWithQuery(items, vmTableHeaders, queryOpts, emptyMsg)
	}
}

// buildPlanVMRows builds flat table rows for all VMs in a single plan.
func buildPlanVMRows(
	ctx context.Context,
	configFlags *genericclioptions.ConfigFlags,
	dynamicClient dynamic.Interface,
	plan *unstructured.Unstructured,
	namespace, inventoryURL string,
	insecureSkipTLS bool,
	sourceCache, targetCache map[string]*inventoryCacheEntry,
) ([]map[string]interface{}, error) {
	planNameStr := plan.GetName()
	planNS := plan.GetNamespace()
	if planNS == "" {
		planNS = namespace
	}

	// Get plan spec VMs
	specVMs, exists, _ := unstructured.NestedSlice(plan.Object, "spec", "vms")
	if !exists || len(specVMs) == 0 {
		return nil, nil
	}

	// Get plan details (status, migration)
	planDetails, _ := status.GetPlanDetails(dynamicClient, planNS, plan, client.MigrationsGVR)

	// Get migration status VMs (if migration exists)
	migrationVMs := buildMigrationVMMap(planDetails)

	// Provider info
	sourceName, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "source", "name")
	destName, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "destination", "name")
	targetNamespace, _, _ := unstructured.NestedString(plan.Object, "spec", "targetNamespace")

	// Fetch source inventory (cached)
	sourceVMs := fetchInventoryVMs(ctx, configFlags, sourceName, planNS, inventoryURL, insecureSkipTLS, sourceCache)

	// Fetch target VMs (cached)
	targetWorkloads := fetchInventoryTargetVMs(ctx, configFlags, destName, planNS, inventoryURL, insecureSkipTLS, targetCache)

	// Build rows
	rows := make([]map[string]interface{}, 0, len(specVMs))
	for _, v := range specVMs {
		specVM, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, _, _ := unstructured.NestedString(specVM, "name")
		vmID, _, _ := unstructured.NestedString(specVM, "id")
		specTargetName, _, _ := unstructured.NestedString(specVM, "targetName")

		// Source inventory lookup
		srcStatus, srcIP := lookupSourceVM(sourceVMs, vmID, vmName)

		// Migration status lookup
		migVM := migrationVMs[vmID]
		progressStr := buildProgressString(migVM)

		// Target name resolution
		tgtDisplayName := resolveTargetName(specTargetName, migVM, vmName)
		tgtDisplay := tgtDisplayName
		if targetNamespace != "" {
			tgtDisplay = targetNamespace + "/" + tgtDisplayName
		}

		// Target inventory lookup
		tgtStatus, tgtIP := lookupTargetWorkload(targetWorkloads, tgtDisplayName)

		planDisplay := planNameStr
		if !planDetails.IsReady {
			planDisplay = planNameStr + " [not ready]"
		}

		row := map[string]interface{}{
			"vm":           vmName,
			"sourceStatus": srcStatus,
			"sourceIP":     srcIP,
			"target":       tgtDisplay,
			"targetIP":     tgtIP,
			"targetStatus": tgtStatus,
			"plan":         planDisplay,
			"planStatus":   planDetails.Status,
			"progress":     progressStr,
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// buildMigrationVMMap indexes migration status VMs by their ID.
func buildMigrationVMMap(planDetails status.PlanDetails) map[string]map[string]interface{} {
	result := map[string]map[string]interface{}{}

	migration := planDetails.RunningMigration
	if migration == nil {
		migration = planDetails.LatestMigration
	}
	if migration == nil {
		return result
	}

	vms, exists, _ := unstructured.NestedSlice(migration.Object, "status", "vms")
	if !exists {
		return result
	}

	for _, v := range vms {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		id, _, _ := unstructured.NestedString(vm, "id")
		if id != "" {
			result[id] = vm
		}
	}

	return result
}

// buildProgressString derives a progress string from a migration status VM.
func buildProgressString(migVM map[string]interface{}) string {
	if migVM == nil {
		return "-"
	}

	phase, _, _ := unstructured.NestedString(migVM, "phase")
	if phase == "" {
		return "-"
	}

	completionStatus := getVMCompletionStatus(migVM)
	if completionStatus == status.StatusSucceeded || completionStatus == status.StatusCompleted {
		return "Completed"
	}
	if completionStatus == status.StatusFailed {
		return "Failed"
	}
	if completionStatus == status.StatusCanceled {
		return "Canceled"
	}

	// Calculate aggregate progress from pipeline
	pipeline, exists, _ := unstructured.NestedSlice(migVM, "pipeline")
	if !exists || len(pipeline) == 0 {
		return phase
	}

	var totalCompleted, totalTotal int64
	for _, p := range pipeline {
		pipePhase, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		progressMap, exists, _ := unstructured.NestedMap(pipePhase, "progress")
		if !exists {
			continue
		}
		completed, ok := toInt64(progressMap["completed"])
		if !ok {
			klog.V(4).Infof("unexpected type %T for progress.completed", progressMap["completed"])
		}
		total, ok := toInt64(progressMap["total"])
		if !ok {
			klog.V(4).Infof("unexpected type %T for progress.total", progressMap["total"])
		}
		totalCompleted += completed
		totalTotal += total
	}

	if totalTotal > 0 {
		pct := float64(totalCompleted) / float64(totalTotal) * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("%s (%.0f%%)", phase, pct)
	}

	return phase
}

// toInt64 converts a JSON-unmarshalled numeric value to int64.
// JSON numbers decoded into interface{} arrive as float64;
// this also handles int64, int, float32, and json.Number.
func toInt64(v interface{}) (int64, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			f, ferr := n.Float64()
			if ferr != nil {
				return 0, false
			}
			return int64(f), true
		}
		return i, true
	default:
		return 0, false
	}
}

// resolveTargetName determines the target VM name from spec, migration, or fallback.
func resolveTargetName(specTargetName string, migVM map[string]interface{}, sourceName string) string {
	if specTargetName != "" {
		return specTargetName
	}
	if migVM != nil {
		newName, _, _ := unstructured.NestedString(migVM, "newName")
		if newName != "" {
			return newName
		}
	}
	return sourceName
}

// fetchInventoryVMs fetches source VMs from inventory and returns a lookup map keyed by VM ID.
func fetchInventoryVMs(
	ctx context.Context,
	configFlags *genericclioptions.ConfigFlags,
	providerName, namespace, inventoryURL string,
	insecureSkipTLS bool,
	cache map[string]*inventoryCacheEntry,
) map[string]map[string]interface{} {
	if providerName == "" || inventoryURL == "" {
		return nil
	}

	cacheKey := namespace + "/" + providerName
	if entry, ok := cache[cacheKey]; ok {
		return entry.vms
	}

	result := &inventoryCacheEntry{}
	cache[cacheKey] = result

	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		klog.V(1).Infof("Failed to get source provider %s: %v", providerName, err)
		result.err = err
		return nil
	}

	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, insecureSkipTLS)
	data, err := providerClient.GetVMs(ctx, 4)
	if err != nil {
		klog.V(1).Infof("Failed to fetch VMs from source provider %s: %v", providerName, err)
		result.err = err
		return nil
	}

	vmMap := map[string]map[string]interface{}{}
	if dataArray, ok := data.([]interface{}); ok {
		for _, item := range dataArray {
			if vm, ok := item.(map[string]interface{}); ok {
				if id, ok := vm["id"].(string); ok && id != "" {
					vmMap[id] = vm
				}
				if name, ok := vm["name"].(string); ok && name != "" {
					vmMap["name:"+name] = vm
				}
			}
		}
	}

	result.vms = vmMap
	return vmMap
}

// fetchInventoryTargetVMs fetches target VMs from inventory and returns a lookup map keyed by name.
// For OpenShift target providers, KubeVirt VirtualMachines are served via the "vms" endpoint.
func fetchInventoryTargetVMs(
	ctx context.Context,
	configFlags *genericclioptions.ConfigFlags,
	providerName, namespace, inventoryURL string,
	insecureSkipTLS bool,
	cache map[string]*inventoryCacheEntry,
) map[string]map[string]interface{} {
	if providerName == "" || inventoryURL == "" {
		return nil
	}

	cacheKey := namespace + "/" + providerName
	if entry, ok := cache[cacheKey]; ok {
		return entry.vms
	}

	result := &inventoryCacheEntry{}
	cache[cacheKey] = result

	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		klog.V(1).Infof("Failed to get target provider %s: %v", providerName, err)
		result.err = err
		return nil
	}

	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, inventoryURL, insecureSkipTLS)
	data, err := providerClient.GetVMs(ctx, 4)
	if err != nil {
		klog.V(1).Infof("Failed to fetch VMs from target provider %s: %v", providerName, err)
		result.err = err
		return nil
	}

	vmMap := map[string]map[string]interface{}{}
	if dataArray, ok := data.([]interface{}); ok {
		for _, item := range dataArray {
			if vm, ok := item.(map[string]interface{}); ok {
				if name, ok := vm["name"].(string); ok && name != "" {
					vmMap[name] = vm
				}
			}
		}
	}

	result.vms = vmMap
	return vmMap
}

// lookupSourceVM looks up a source VM in the inventory by ID (preferred) or name.
func lookupSourceVM(sourceVMs map[string]map[string]interface{}, vmID, vmName string) (statusStr, ip string) {
	if sourceVMs == nil {
		return "-", "-"
	}

	vm := sourceVMs[vmID]
	if vm == nil {
		vm = sourceVMs["name:"+vmName]
	}
	if vm == nil {
		return "Not Found", "-"
	}

	return extractPowerStatus(vm), extractIP(vm)
}

// lookupTargetWorkload looks up a target workload in the inventory by name.
func lookupTargetWorkload(targetWorkloads map[string]map[string]interface{}, targetName string) (statusStr, ip string) {
	if targetWorkloads == nil {
		return "-", "-"
	}

	wl := targetWorkloads[targetName]
	if wl == nil {
		return "Not Found", "-"
	}

	return extractPowerStatus(wl), extractIP(wl)
}

// extractPowerStatus extracts a human-readable power status from a VM/workload map.
func extractPowerStatus(vm map[string]interface{}) string {
	// Check "powerState" (ovirt, vsphere)
	if ps, ok := vm["powerState"].(string); ok && ps != "" {
		lower := strings.ToLower(ps)
		if strings.Contains(lower, "on") || strings.Contains(lower, "up") || strings.Contains(lower, "running") {
			return "Running"
		}
		return "Stopped"
	}

	// Check "status" (OpenShift workloads)
	if st, ok := vm["status"].(string); ok && st != "" {
		lower := strings.ToLower(st)
		if strings.Contains(lower, "running") {
			return "Running"
		}
		if strings.Contains(lower, "stopped") || strings.Contains(lower, "off") {
			return "Stopped"
		}
		return st
	}

	// Check nested object.status.printableStatus (KubeVirt VirtualMachine workloads)
	if ps, found, _ := unstructured.NestedString(vm, "object", "status", "printableStatus"); found && ps != "" {
		lower := strings.ToLower(ps)
		if strings.Contains(lower, "running") {
			return "Running"
		}
		if strings.Contains(lower, "stopped") || strings.Contains(lower, "off") || strings.Contains(lower, "halted") {
			return "Stopped"
		}
		return ps
	}

	// Check nested object.status.phase (OpenShift workloads with object wrapper)
	if phase, found, _ := unstructured.NestedString(vm, "object", "status", "phase"); found && phase != "" {
		lower := strings.ToLower(phase)
		if strings.Contains(lower, "running") {
			return "Running"
		}
		if strings.Contains(lower, "stopped") || strings.Contains(lower, "off") {
			return "Stopped"
		}
		return phase
	}

	// EC2: State.Name
	if state, found, _ := unstructured.NestedString(vm, "State", "Name"); found && state != "" {
		if strings.ToLower(state) == "running" {
			return "Running"
		}
		return "Stopped"
	}

	return "-"
}

// extractIP extracts an IP address from a VM/workload map.
func extractIP(vm map[string]interface{}) string {
	// Direct ipAddress field (ovirt, vsphere detail)
	if ip, ok := vm["ipAddress"].(string); ok && ip != "" {
		return ip
	}

	// EC2: PublicIpAddress or PrivateIpAddress
	if ip, ok := vm["PublicIpAddress"].(string); ok && ip != "" {
		return ip
	}
	if ip, ok := vm["PrivateIpAddress"].(string); ok && ip != "" {
		return ip
	}

	// Network interfaces array (various providers)
	if nics, ok := vm["nics"].([]interface{}); ok {
		for _, n := range nics {
			nic, ok := n.(map[string]interface{})
			if !ok {
				continue
			}
			if ip, ok := nic["ipAddress"].(string); ok && ip != "" {
				return ip
			}
			if ips, ok := nic["ipAddresses"].([]interface{}); ok {
				for _, ipVal := range ips {
					if ip, ok := ipVal.(string); ok && ip != "" {
						return ip
					}
				}
			}
		}
	}

	// OpenShift workload: nested object.status.interfaces
	if interfaces, found, _ := unstructured.NestedSlice(vm, "object", "status", "interfaces"); found {
		for _, iface := range interfaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			if ip, ok := ifaceMap["ipAddress"].(string); ok && ip != "" {
				return ip
			}
		}
	}

	return "-"
}
