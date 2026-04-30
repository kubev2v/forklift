package plan

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	planutil "github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Describe describes a migration plan.
func Describe(configFlags *genericclioptions.ConfigFlags, name, namespace string, withVMs bool, useUTC bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan: %v", err)
	}

	planDetails, _ := status.GetPlanDetails(c, namespace, plan, client.MigrationsGVR)

	b := describe.NewBuilder("MIGRATION PLAN")

	// Basic information (implicit first section)
	b.Field("Name", plan.GetName())
	b.Field("Namespace", plan.GetNamespace())
	b.Field("Created", output.FormatTimestamp(plan.GetCreationTimestamp().Time, useUTC))

	archived, exists, _ := unstructured.NestedBool(plan.Object, "spec", "archived")
	if exists {
		b.Field("Archived", fmt.Sprintf("%t", archived))
	} else {
		b.Field("Archived", "false")
	}

	b.FieldC("Ready", fmt.Sprintf("%t", planDetails.IsReady), output.ColorizeBooleanString)
	b.FieldC("Status", planDetails.Status, output.ColorizeStatus)

	// Specification
	buildSpecSection(b, plan)

	// Mappings
	networkMapping, _, _ := unstructured.NestedString(plan.Object, "spec", "map", "network", "name")
	storageMapping, _, _ := unstructured.NestedString(plan.Object, "spec", "map", "storage", "name")
	migrationType, _, _ := unstructured.NestedString(plan.Object, "spec", "type")
	buildMappingsSection(b, networkMapping, storageMapping, migrationType)

	// Running / Latest migration
	buildMigrationSection(b, "RUNNING MIGRATION", planDetails.RunningMigration, planDetails)
	if planDetails.RunningMigration == nil {
		buildMigrationSection(b, "LATEST MIGRATION", planDetails.LatestMigration, planDetails)
	}

	// Mapping details
	buildMappingDetailsSectionImpl(b, c, namespace, "NETWORK MAPPING DETAILS", networkMapping, true)
	buildMappingDetailsSectionImpl(b, c, namespace, "STORAGE MAPPING DETAILS", storageMapping, false)

	// Conditions
	buildConditionsSection(b, plan)

	// VMs
	if withVMs {
		migration := planDetails.RunningMigration
		if migration == nil {
			migration = planDetails.LatestMigration
		}
		buildVMsSection(b, plan, migration, useUTC)
	}

	return describe.Print(b.Build(), outputFormat)
}

func buildSpecSection(b *describe.Builder, plan *unstructured.Unstructured) {
	source, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "source", "name")
	target, _, _ := unstructured.NestedString(plan.Object, "spec", "provider", "destination", "name")
	targetNamespace, _, _ := unstructured.NestedString(plan.Object, "spec", "targetNamespace")
	transferNetwork, _, _ := unstructured.NestedString(plan.Object, "spec", "transferNetwork", "name")
	description, _, _ := unstructured.NestedString(plan.Object, "spec", "description")
	preserveCPUModel, _, _ := unstructured.NestedBool(plan.Object, "spec", "preserveClusterCPUModel")
	preserveStaticIPs, _, _ := unstructured.NestedBool(plan.Object, "spec", "preserveStaticIPs")
	enableNestedVirt, enableNestedVirtExists, _ := unstructured.NestedBool(plan.Object, "spec", "enableNestedVirtualization")
	xfsCompatibility, _, _ := unstructured.NestedBool(plan.Object, "spec", "xfsCompatibility")

	migrationType := "cold"
	if v, exists, _ := unstructured.NestedString(plan.Object, "spec", "type"); exists && v != "" {
		migrationType = v
	} else if warm, exists, _ := unstructured.NestedBool(plan.Object, "spec", "warm"); exists && warm {
		migrationType = "warm"
	}

	b.Section("SPECIFICATION")
	b.SubSection("Providers")
	b.Field("Source", source)
	b.Field("Target", target)
	b.EndSubSection()

	b.SubSection("Migration Settings")
	b.Field("Target Namespace", targetNamespace)
	b.Field("Migration Type", migrationType)
	if transferNetwork != "" {
		b.Field("Transfer Network", transferNetwork)
	}
	b.EndSubSection()

	b.SubSection("Advanced Settings")
	b.FieldC("Preserve CPU Model", fmt.Sprintf("%t", preserveCPUModel), output.ColorizeBooleanString)
	b.FieldC("Preserve Static IPs", fmt.Sprintf("%t", preserveStaticIPs), output.ColorizeBooleanString)
	if enableNestedVirtExists {
		b.FieldC("Nested Virtualization", fmt.Sprintf("%t", enableNestedVirt), output.ColorizeBooleanString)
	} else {
		b.Field("Nested Virtualization", "auto-detect")
	}
	b.FieldC("XFS Compatibility", fmt.Sprintf("%t", xfsCompatibility), output.ColorizeBooleanString)
	b.EndSubSection()

	if description != "" {
		b.Field("Description", description)
	}
}

func buildMappingsSection(b *describe.Builder, networkMapping, storageMapping, migrationType string) {
	b.Section("MAPPINGS")

	if networkMapping != "" {
		b.Field("Network Mapping", networkMapping)
	} else {
		b.FieldC("Network Mapping", "Not specified", output.Red)
	}

	if storageMapping != "" {
		b.Field("Storage Mapping", storageMapping)
	} else if migrationType == "conversion" {
		b.FieldC("Storage Mapping", "Not required (conversion-only)", output.Green)
	} else {
		b.FieldC("Storage Mapping", "Not specified", output.Red)
	}
}

func buildMigrationSection(b *describe.Builder, title string, migration *unstructured.Unstructured, details status.PlanDetails) {
	if migration == nil {
		return
	}

	b.Section(title)
	b.Field("Name", migration.GetName())
	b.Field("Total VMs", fmt.Sprintf("%d", details.VMStats.Total))
	b.Field("Completed", fmt.Sprintf("%d", details.VMStats.Completed))
	b.FieldC("Succeeded", fmt.Sprintf("%d", details.VMStats.Succeeded), output.Green)
	b.FieldC("Failed", fmt.Sprintf("%d", details.VMStats.Failed), colorIfNonZero(details.VMStats.Failed, output.Red))
	b.FieldC("Canceled", fmt.Sprintf("%d", details.VMStats.Canceled), colorIfNonZero(details.VMStats.Canceled, output.Yellow))

	if details.DiskProgress.Total > 0 {
		pct := float64(details.DiskProgress.Completed) / float64(details.DiskProgress.Total) * 100
		progressText := fmt.Sprintf("%.1f%% (%d/%d GB)", pct, details.DiskProgress.Completed/1024, details.DiskProgress.Total/1024)
		colorFn := output.Yellow
		if pct >= 100 {
			colorFn = output.Green
		}
		b.FieldC("Disk Transfer", progressText, colorFn)
	}
}

func buildMappingDetailsSectionImpl(b *describe.Builder, c dynamic.Interface, namespace, title, mappingName string, isNetwork bool) {
	var gvr = client.StorageMapGVR
	if isNetwork {
		gvr = client.NetworkMapGVR
	}

	m, err := c.Resource(gvr).Namespace(namespace).Get(context.TODO(), mappingName, metav1.GetOptions{})
	if err != nil {
		return
	}

	pairs, exists, _ := unstructured.NestedSlice(m.Object, "spec", "map")
	if !exists || len(pairs) == 0 {
		return
	}

	b.Section(title)

	headers := []describe.TableColumn{
		{Display: "SOURCE", Key: "source"},
		{Display: "DESTINATION", Key: "destination"},
	}

	rows := make([]map[string]string, 0, len(pairs))
	for _, entry := range pairs {
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

func buildConditionsSection(b *describe.Builder, plan *unstructured.Unstructured) {
	conditions, exists, _ := unstructured.NestedSlice(plan.Object, "status", "conditions")
	if !exists || len(conditions) == 0 {
		return
	}

	b.Section("CONDITIONS")

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

func buildVMsSection(b *describe.Builder, plan *unstructured.Unstructured, migration *unstructured.Unstructured, useUTC bool) {
	specVMs, exists, err := unstructured.NestedSlice(plan.Object, "spec", "vms")
	if err != nil || !exists || len(specVMs) == 0 {
		b.Section("VIRTUAL MACHINES")
		b.FieldC("Status", "No VMs specified in the plan", output.Red)
		return
	}

	// Build a vmID -> status map from the migration's status.vms
	vmStatusMap := buildVMStatusMap(migration)

	b.Section("VIRTUAL MACHINES")
	b.Field("VM Count", fmt.Sprintf("%d", len(specVMs)))

	for i, v := range specVMs {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, _, _ := unstructured.NestedString(vm, "name")
		vmID, _, _ := unstructured.NestedString(vm, "id")
		targetName, _, _ := unstructured.NestedString(vm, "targetName")
		instanceType, _, _ := unstructured.NestedString(vm, "instanceType")
		rootDisk, _, _ := unstructured.NestedString(vm, "rootDisk")
		targetPowerState, _, _ := unstructured.NestedString(vm, "targetPowerState")
		pvcNameTemplate, _, _ := unstructured.NestedString(vm, "pvcNameTemplate")
		volumeNameTemplate, _, _ := unstructured.NestedString(vm, "volumeNameTemplate")
		networkNameTemplate, _, _ := unstructured.NestedString(vm, "networkNameTemplate")
		enableNestedVirt, enableNestedVirtExists, _ := unstructured.NestedBool(vm, "enableNestedVirtualization")
		hooks, _, _ := unstructured.NestedSlice(vm, "hooks")
		luks, _, _ := unstructured.NestedMap(vm, "luks")

		b.SubSection(fmt.Sprintf("VM #%d", i+1))

		b.Field("Name", stringOrDefault(vmName, "-"))
		b.FieldC("ID", stringOrDefault(vmID, "-"), output.Cyan)
		if targetName != "" {
			b.FieldC("Target Name", targetName, output.Green)
		}

		// Migration status for this VM
		if vmStatus, ok := vmStatusMap[vmID]; ok {
			addVMMigrationStatus(b, vmStatus, useUTC)
		}

		if instanceType != "" {
			b.Field("Instance Type", instanceType)
		}
		if rootDisk != "" {
			b.FieldC("Root Disk", rootDisk, output.Blue)
		}
		if targetPowerState != "" {
			b.FieldC("Target Power State", targetPowerState, output.ColorizePowerState)
		}

		if pvcNameTemplate != "" {
			b.Field("PVC Template", pvcNameTemplate)
		}
		if volumeNameTemplate != "" {
			b.Field("Volume Template", volumeNameTemplate)
		}
		if networkNameTemplate != "" {
			b.Field("Network Template", networkNameTemplate)
		}
		if enableNestedVirtExists {
			b.FieldC("Nested Virtualization", fmt.Sprintf("%t", enableNestedVirt), output.ColorizeBooleanString)
		}

		if len(hooks) > 0 {
			for j, h := range hooks {
				hook, ok := h.(map[string]interface{})
				if !ok {
					continue
				}
				hookName, _, _ := unstructured.NestedString(hook, "name")
				hookKind, _, _ := unstructured.NestedString(hook, "kind")
				hookNS, _, _ := unstructured.NestedString(hook, "namespace")
				label := fmt.Sprintf("Hook %d", j+1)
				val := stringOrDefault(hookName, "-")
				if hookKind != "" || hookNS != "" {
					val += fmt.Sprintf(" (%s/%s)", stringOrDefault(hookNS, "default"), stringOrDefault(hookKind, "Hook"))
				}
				b.FieldC(label, val, output.Green)
			}
		} else {
			b.Field("Hooks", "None")
		}

		if len(luks) > 0 {
			luksName, _, _ := unstructured.NestedString(luks, "name")
			if luksName != "" {
				b.Field("LUKS Secret", luksName)
			}
		} else {
			b.Field("Disk Encryption", "None")
		}

		b.EndSubSection()
	}
}

// buildVMStatusMap extracts the VM status entries from a migration and returns
// them as a map keyed by VM ID for quick lookup.
func buildVMStatusMap(migration *unstructured.Unstructured) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
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

// addVMMigrationStatus appends migration status fields for a single VM.
func addVMMigrationStatus(b *describe.Builder, vmStatus map[string]interface{}, useUTC bool) {
	phase, _, _ := unstructured.NestedString(vmStatus, "phase")
	started, _, _ := unstructured.NestedString(vmStatus, "started")
	completed, _, _ := unstructured.NestedString(vmStatus, "completed")

	b.FieldC("Migration Phase", stringOrDefault(phase, "Pending"), output.ColorizeStatus)

	if started != "" {
		b.Field("Migration Started", planutil.FormatTime(started, useUTC))
	}
	if completed != "" {
		b.Field("Migration Completed", planutil.FormatTime(completed, useUTC))
	}

	// Summarise pipeline progress as a compact line per phase
	pipeline, exists, _ := unstructured.NestedSlice(vmStatus, "pipeline")
	if !exists || len(pipeline) == 0 {
		return
	}

	headers := []describe.TableColumn{
		{Display: "STEP", Key: "step"},
		{Display: "STATUS", Key: "status", ColorFunc: output.ColorizeStatus},
		{Display: "PROGRESS", Key: "progress", ColorFunc: output.ColorizeProgress},
	}

	rows := make([]map[string]string, 0, len(pipeline))
	for _, p := range pipeline {
		step, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		stepName, _, _ := unstructured.NestedString(step, "name")
		stepPhase, _, _ := unstructured.NestedString(step, "phase")

		progress := "-"
		if pm, exists, _ := unstructured.NestedMap(step, "progress"); exists {
			comp, _, _ := unstructured.NestedInt64(pm, "completed")
			total, _, _ := unstructured.NestedInt64(pm, "total")
			if total > 0 {
				progress = fmt.Sprintf("%.1f%%", float64(comp)/float64(total)*100)
			}
		}

		rows = append(rows, map[string]string{
			"step":     stepName,
			"status":   stepPhase,
			"progress": progress,
		})
	}

	b.Table(headers, rows)
}

// formatMappingEntry formats a single mapping entry (source or destination) as a string.
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
	if dt, ok := entry["type"].(string); ok && dt != "" {
		parts = append(parts, "Type: "+dt)
	}
	if ns, ok := entry["namespace"].(string); ok && ns != "" {
		parts = append(parts, "Namespace: "+ns)
	}
	if multus, found, _ := unstructured.NestedMap(entry, "multus"); found {
		if nn, ok := multus["networkName"].(string); ok && nn != "" {
			parts = append(parts, "Multus Network: "+nn)
		}
	}

	return strings.Join(parts, ", ")
}

func stringOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func colorIfNonZero(n int, colorFn func(string) string) func(string) string {
	if n > 0 {
		return colorFn
	}
	return nil
}
