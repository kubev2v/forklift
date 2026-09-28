package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	planstatus "github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Cutover sets the cutover time for a warm migration. If vmRefs is non-empty,
// only the listed VMs (matched by name or ID against the plan's VM list) get
// the cutover time; otherwise the plan-wide cutover time is set.
func Cutover(configFlags *genericclioptions.ConfigFlags, planName, namespace string, cutoverTime *time.Time, vmRefs []string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	planObj, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), planName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan '%s': %v", planName, err)
	}

	// Check if the plan is warm (handles both spec.type and legacy spec.warm)
	if !planstatus.IsWarmMigration(planObj) {
		return fmt.Errorf("plan '%s' is not configured for warm migration", planName)
	}

	// Find the running migration for this plan
	runningMigration, _, err := planstatus.GetRunningMigration(c, namespace, planObj, client.MigrationsGVR)
	if err != nil {
		return err
	}
	if runningMigration == nil {
		return fmt.Errorf("no running migration found for plan '%s'", planName)
	}

	// If no cutover time provided, use current time
	if cutoverTime == nil {
		now := time.Now()
		cutoverTime = &now
	}

	// Format the cutover time as RFC3339 (the format Kubernetes uses for metav1.Time)
	cutoverTimeRFC3339 := cutoverTime.Format(time.RFC3339)

	if len(vmRefs) > 0 {
		return cutoverVMs(c, planObj, runningMigration, planName, namespace, vmRefs, cutoverTimeRFC3339)
	}

	// Prepare the patch to set the plan-wide cutover field
	patchObject := map[string]interface{}{
		"spec": map[string]interface{}{
			"cutover": cutoverTimeRFC3339,
		},
	}

	// Convert the patch to JSON
	patchBytes, err := json.Marshal(patchObject)
	if err != nil {
		return fmt.Errorf("failed to create patch: %v", err)
	}

	// Apply the patch to the migration
	_, err = c.Resource(client.MigrationsGVR).Namespace(namespace).Patch(
		context.TODO(),
		runningMigration.GetName(),
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update migration with cutover time: %v", err)
	}

	fmt.Printf("Successfully set cutover time to %s for plan '%s'\n", cutoverTimeRFC3339, planName)
	return nil
}

// cutoverVMs resolves vmRefs (names or IDs) against the plan's VM list and
// patches spec.vmCutover on the running migration with the resolved entries,
// merging with any existing per-VM cutover entries.
func cutoverVMs(c dynamic.Interface, planObj, runningMigration *unstructured.Unstructured, planName, namespace string, vmRefs []string, cutoverTimeRFC3339 string) error {
	resolvedIDs, resolvedNames, err := resolveVMRefs(planObj, vmRefs)
	if err != nil {
		return err
	}

	// Read the current per-VM cutover list to avoid overwriting other entries
	currentVMCutover, _, _ := unstructured.NestedSlice(runningMigration.Object, "spec", "vmCutover")

	newVMCutover := make([]vmCutoverEntry, 0, len(resolvedIDs))
	for _, id := range resolvedIDs {
		newVMCutover = append(newVMCutover, vmCutoverEntry{ID: id, Cutover: cutoverTimeRFC3339})
	}

	mergedVMCutover := mergeVMCutovers(currentVMCutover, newVMCutover)

	patchObject := map[string]interface{}{
		"spec": map[string]interface{}{
			"vmCutover": mergedVMCutover,
		},
	}

	patchBytes, err := json.Marshal(patchObject)
	if err != nil {
		return fmt.Errorf("failed to create patch: %v", err)
	}

	_, err = c.Resource(client.MigrationsGVR).Namespace(namespace).Patch(
		context.TODO(),
		runningMigration.GetName(),
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update migration with per-VM cutover time: %v", err)
	}

	fmt.Printf("Successfully set cutover time to %s for VMs [%s] in plan '%s'\n",
		cutoverTimeRFC3339, strings.Join(resolvedNames, ", "), planName)
	return nil
}

// resolveVMRefs resolves a list of VM names or IDs against the plan's spec.vms
// list, returning the resolved IDs and a display name for each. A ref is first
// matched against VM names, then falls back to matching against VM IDs directly.
func resolveVMRefs(planObj *unstructured.Unstructured, vmRefs []string) (ids []string, names []string, err error) {
	planVMs, found, err := unstructured.NestedSlice(planObj.Object, "spec", "vms")
	if err != nil || !found {
		return nil, nil, fmt.Errorf("failed to get VMs from plan: %v", err)
	}

	vmNameToID := make(map[string]string)
	vmIDToName := make(map[string]string)
	for _, vmObj := range planVMs {
		vm, ok := vmObj.(map[string]interface{})
		if !ok {
			continue
		}
		vmName, _ := vm["name"].(string)
		vmID, _ := vm["id"].(string)
		if vmID == "" {
			continue
		}
		if vmName != "" {
			vmNameToID[vmName] = vmID
		}
		vmIDToName[vmID] = vmName
	}

	var notFound []string
	for _, ref := range vmRefs {
		if id, ok := vmNameToID[ref]; ok {
			ids = append(ids, id)
			names = append(names, ref)
			continue
		}
		if name, ok := vmIDToName[ref]; ok {
			ids = append(ids, ref)
			if name != "" {
				names = append(names, name)
			} else {
				names = append(names, ref)
			}
			continue
		}
		notFound = append(notFound, ref)
	}

	if len(notFound) > 0 {
		return nil, nil, fmt.Errorf("the following VMs were not found in plan: %v", notFound)
	}

	return ids, names, nil
}

// vmCutoverEntry mirrors the upstream forklift VMCutover CRD type.
type vmCutoverEntry struct {
	ID      string
	Cutover string
}

// mergeVMCutovers merges the existing spec.vmCutover entries (as read from the
// migration) with new entries, keyed by VM ID so new values override existing
// ones for the same VM while leaving other VMs' entries untouched.
func mergeVMCutovers(existing []interface{}, new []vmCutoverEntry) []interface{} {
	uniqueByID := make(map[string]vmCutoverEntry)

	for _, entryObj := range existing {
		entry, ok := entryObj.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := entry["id"].(string)
		cutover, _ := entry["cutover"].(string)
		if id == "" {
			continue
		}
		uniqueByID[id] = vmCutoverEntry{ID: id, Cutover: cutover}
	}

	for _, entry := range new {
		if entry.ID == "" {
			continue
		}
		uniqueByID[entry.ID] = entry
	}

	result := make([]interface{}, 0, len(uniqueByID))
	for _, entry := range uniqueByID {
		result = append(result, map[string]interface{}{
			"id":      entry.ID,
			"cutover": entry.Cutover,
		})
	}

	return result
}
