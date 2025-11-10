package plan

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Archive sets the archived flag on a plan
func Archive(ctx context.Context, configFlags *genericclioptions.ConfigFlags, planName, namespace string, archived bool) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	_, err = c.Resource(client.PlansGVR).Namespace(namespace).Get(ctx, planName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan '%s': %v", planName, err)
	}

	// Create a patch to update the archived field
	patchObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"archived": archived,
		},
	}

	patchBytes, err := json.Marshal(patchObj)
	if err != nil {
		return fmt.Errorf("failed to create patch: %v", err)
	}

	// Apply the patch
	_, err = c.Resource(client.PlansGVR).Namespace(namespace).Patch(
		ctx,
		planName,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update plan: %v", err)
	}

	action := "archived"
	if !archived {
		action = "unarchived"
	}

	fmt.Printf("Plan '%s' %s\n", planName, action)
	return nil
}
