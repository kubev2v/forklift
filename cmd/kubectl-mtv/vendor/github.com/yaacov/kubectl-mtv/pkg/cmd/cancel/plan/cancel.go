package plan

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Cancel cancels specific VMs in a running migration
func Cancel(configFlags *genericclioptions.ConfigFlags, planName string, namespace string, vmNames []string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	planObj, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), planName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan '%s': %v", planName, err)
	}

	// Validate that VM names exist in the plan
	planVMs, found, err := unstructured.NestedSlice(planObj.Object, "spec", "vms")
	if err != nil || !found {
		return fmt.Errorf("failed to get VMs from plan: %v", err)
	}

	vmNameToIDMap := make(map[string]string)
	for _, vmObj := range planVMs {
		vm, ok := vmObj.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, ok := vm["name"].(string)
		if !ok || vmName == "" {
			continue
		}

		vmID, ok := vm["id"].(string)
		if !ok || vmID == "" {
			continue
		}

		vmNameToIDMap[vmName] = vmID
	}

	// Check if requested VM names exist in the plan
	var invalidVMs []string
	var validVMs []string
	for _, vmName := range vmNames {
		if _, exists := vmNameToIDMap[vmName]; !exists {
			invalidVMs = append(invalidVMs, vmName)
		} else {
			validVMs = append(validVMs, vmName)
		}
	}

	if len(invalidVMs) > 0 {
		return fmt.Errorf("the following VMs were not found in plan '%s': %v", planName, invalidVMs)
	}

	// Find the running migration for this plan
	runningMigration, _, err := status.GetRunningMigration(c, namespace, planObj, client.MigrationsGVR)
	if err != nil {
		return err
	}
	if runningMigration == nil {
		return fmt.Errorf("no running migration found for plan '%s'", planName)
	}

	// Prepare the VM references to cancel
	var cancelVMs []ref.Ref
	for _, vmName := range validVMs {
		cancelVMs = append(cancelVMs, ref.Ref{
			Name: vmName,
			ID:   vmNameToIDMap[vmName],
		})
	}

	// Create a patch to update the cancel field
	// First, get the current cancel list to avoid overwriting it
	currentCancelVMs, _, _ := unstructured.NestedSlice(runningMigration.Object, "spec", "cancel")

	// Convert current cancel VMs to ref.Ref structures
	var existingCancelVMs []ref.Ref
	for _, vmObj := range currentCancelVMs {
		vm, ok := vmObj.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, _ := vm["name"].(string)
		vmID, _ := vm["id"].(string)

		existingCancelVMs = append(existingCancelVMs, ref.Ref{
			Name: vmName,
			ID:   vmID,
		})
	}

	// Merge existing and new cancel VMs, avoiding duplicates
	mergedCancelVMs := mergeCancelVMs(existingCancelVMs, cancelVMs)

	// Prepare the patch
	patchObject := map[string]interface{}{
		"spec": map[string]interface{}{
			"cancel": mergedCancelVMs,
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
		return fmt.Errorf("failed to update migration with canceled VMs: %v", err)
	}

	fmt.Printf("Successfully requested cancellation for VMs in plan '%s': %v\n", planName, validVMs)
	return nil
}

// mergeCancelVMs merges two slices of ref.Ref, avoiding duplicates based on VM ID
func mergeCancelVMs(existing, new []ref.Ref) []interface{} {
	// Create a map to track unique VMs by ID
	uniqueVMs := make(map[string]ref.Ref)

	// Add existing VMs to the map
	for _, vm := range existing {
		if vm.ID != "" {
			uniqueVMs[vm.ID] = vm
		}
	}

	// Add new VMs to the map (will override any duplicates)
	for _, vm := range new {
		if vm.ID != "" {
			uniqueVMs[vm.ID] = vm
		}
	}

	// Convert the map back to a slice of interface{} for unstructured
	result := make([]interface{}, 0, len(uniqueVMs))
	for _, vm := range uniqueVMs {
		// Convert ref.Ref to map for unstructured
		vmMap := map[string]interface{}{
			"name": vm.Name,
			"id":   vm.ID,
		}
		result = append(result, vmMap)
	}

	return result
}
