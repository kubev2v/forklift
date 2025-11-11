package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Cutover sets the cutover time for a warm migration
func Cutover(configFlags *genericclioptions.ConfigFlags, planName, namespace string, cutoverTime *time.Time) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	planObj, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), planName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan '%s': %v", planName, err)
	}

	// Check if the plan is warm
	warm, exists, err := unstructured.NestedBool(planObj.Object, "spec", "warm")
	if err != nil || !exists || !warm {
		return fmt.Errorf("plan '%s' is not configured for warm migration", planName)
	}

	// Find the running migration for this plan
	runningMigration, _, err := status.GetRunningMigration(c, namespace, planObj, client.MigrationsGVR)
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

	// Prepare the patch to set the cutover field
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
