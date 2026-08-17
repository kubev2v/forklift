package diagnostics

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CollectConversions lists Conversion CRs associated with the plan and extracts
// phase, message, and pod references. Results are sorted newest-first by creation timestamp.
func CollectConversions(ctx context.Context, dynClient dynamic.Interface, namespace, planName, vmID string) []ConversionInfo {
	selector := fmt.Sprintf("plan-name=%s", planName)

	convList, err := dynClient.Resource(client.ConversionsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(convList.Items) == 0 {
		return nil
	}

	// Sort newest-first so the caller gets the latest conversion at index 0
	sort.Slice(convList.Items, func(i, j int) bool {
		return convList.Items[i].GetCreationTimestamp().Time.After(convList.Items[j].GetCreationTimestamp().Time)
	})

	var results []ConversionInfo
	for _, conv := range convList.Items {
		// If vmID filter is provided, check if this conversion matches
		if vmID != "" {
			specVMID, _, _ := unstructured.NestedString(conv.Object, "spec", "vm", "id")
			if specVMID != vmID {
				continue
			}
		}

		info := ConversionInfo{
			Name: conv.GetName(),
		}

		info.Phase, _, _ = unstructured.NestedString(conv.Object, "status", "phase")
		info.Message, _, _ = unstructured.NestedString(conv.Object, "status", "message")
		info.PodName, _, _ = unstructured.NestedString(conv.Object, "status", "pod", "name")

		results = append(results, info)
	}
	return results
}
