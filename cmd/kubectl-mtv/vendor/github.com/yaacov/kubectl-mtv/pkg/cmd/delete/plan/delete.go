package plan

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/archive/plan"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Delete removes a plan by name from the cluster
func Delete(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, skipArchive, cleanAll bool) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Patch the plan to add deleteVmOnFailMigration=true if cleanAll is true
	if cleanAll {
		fmt.Printf("Clean-all mode enabled for plan '%s'\n", name)

		// Patch the plan to add deleteVmOnFailMigration=true
		fmt.Printf("Patching plan '%s' to enable VM deletion on failed migration...\n", name)
		err = patchPlanDeleteVmOnFailMigration(ctx, c, name, namespace)
		if err != nil {
			return fmt.Errorf("failed to patch plan: %v", err)
		}

		fmt.Printf("Plan '%s' patched with deleteVmOnFailMigration=true\n", name)
	}

	// Archive the plan if not skipped
	if skipArchive {
		fmt.Printf("Skipping archive and deleting plan '%s' immediately...\n", name)
	} else {
		// Archive the plan
		err = plan.Archive(ctx, configFlags, name, namespace, true)
		if err != nil {
			return fmt.Errorf("failed to archive plan: %v", err)
		}

		// Wait for the Archived condition to be true
		fmt.Printf("Waiting for plan '%s' to be archived...\n", name)
		err = waitForArchivedCondition(ctx, c, name, namespace, 60)
		if err != nil {
			return err
		}

	}

	// Delete the plan
	err = c.Resource(client.PlansGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete plan: %v", err)
	}

	fmt.Printf("Plan '%s' deleted from namespace '%s'\n", name, namespace)
	return nil
}

// waitForArchivedCondition waits for a plan to reach the Archived condition with a timeout
func waitForArchivedCondition(ctx context.Context, c dynamic.Interface, name, namespace string, timeoutSec int) error {
	// Set timeout based on provided seconds
	timeout := time.Duration(timeoutSec) * time.Second
	startTime := time.Now()

	for {
		// Check if we've exceeded the timeout
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for plan '%s' to be archived after %v", name, timeout)
		}

		plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get plan: %v", err)
		}

		conditions, exists, err := unstructured.NestedSlice(plan.Object, "status", "conditions")
		if err != nil || !exists {
			return fmt.Errorf("failed to get plan conditions: %v", err)
		}

		archived := false
		for _, condition := range conditions {
			cond, ok := condition.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(cond, "type")
			condStatus, _, _ := unstructured.NestedString(cond, "status")

			if condType == "Archived" && condStatus == "True" {
				archived = true
				break
			}
		}

		if archived {
			break
		}

		// Wait before checking again
		time.Sleep(2 * time.Second)
	}
	return nil
}

// patchPlanDeleteVmOnFailMigration patches a plan to add deleteVmOnFailMigration=true
func patchPlanDeleteVmOnFailMigration(ctx context.Context, c dynamic.Interface, name, namespace string) error {
	// Create patch data
	patchSpec := map[string]interface{}{
		"deleteVmOnFailMigration": true,
	}

	patchData := map[string]interface{}{
		"spec": patchSpec,
	}

	patchBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, &unstructured.Unstructured{Object: patchData})
	if err != nil {
		return fmt.Errorf("failed to encode patch data: %v", err)
	}

	// Apply the patch
	_, err = c.Resource(client.PlansGVR).Namespace(namespace).Patch(
		ctx,
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch plan: %v", err)
	}

	return nil
}
