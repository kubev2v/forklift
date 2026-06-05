package mapping

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// patchNetworkMapping patches an existing network mapping
func patchNetworkMapping(configFlags *genericclioptions.ConfigFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL string) error {
	klog.V(2).Infof("Patching network mapping '%s' in namespace '%s'", name, namespace)

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the existing mapping
	existingMapping, err := dynamicClient.Resource(client.NetworkMapGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get network mapping '%s': %v", name, err)
	}

	// Extract source provider for pair resolution
	sourceProviderName, sourceProviderNamespace, err := getSourceProviderFromMapping(existingMapping)
	if err != nil {
		return fmt.Errorf("failed to get source provider from mapping: %v", err)
	}

	if sourceProviderNamespace != "" {
		klog.V(2).Infof("Using source provider '%s/%s' for network pair resolution", sourceProviderNamespace, sourceProviderName)
	} else {
		klog.V(2).Infof("Using source provider '%s' for network pair resolution", sourceProviderName)
	}

	// Extract the existing network pairs from the unstructured mapping
	// Work with unstructured data throughout to avoid reflection issues
	currentPairs, found, err := unstructured.NestedSlice(existingMapping.Object, "spec", "map")
	if err != nil {
		return fmt.Errorf("failed to extract existing mapping pairs: %v", err)
	}
	if !found {
		currentPairs = []interface{}{}
	}

	// Work with unstructured pairs to avoid conversion issues
	workingPairs := make([]interface{}, len(currentPairs))
	copy(workingPairs, currentPairs)
	klog.V(3).Infof("Current mapping has %d network pairs", len(workingPairs))

	// Process removals first
	if removePairs != "" {
		sourcesToRemove := parseSourcesToRemove(removePairs)
		klog.V(2).Infof("Removing %d network pairs from mapping", len(sourcesToRemove))
		workingPairs = removeSourceFromUnstructuredPairs(workingPairs, sourcesToRemove)
		klog.V(2).Infof("Successfully removed network pairs from mapping '%s'", name)
	}

	// Process additions
	if addPairs != "" {
		klog.V(2).Infof("Adding network pairs to mapping: %s", addPairs)
		newPairs, err := mapping.ParseNetworkPairs(addPairs, sourceProviderNamespace, configFlags, sourceProviderName, inventoryURL)
		if err != nil {
			return fmt.Errorf("failed to parse add-pairs: %v", err)
		}

		// Convert new pairs to unstructured format
		var newUnstructuredPairs []interface{}
		for _, pair := range newPairs {
			pairMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pair)
			if err != nil {
				klog.V(2).Infof("Warning: Failed to convert new pair to unstructured, skipping: %v", err)
				continue
			}
			newUnstructuredPairs = append(newUnstructuredPairs, pairMap)
		}

		if len(newUnstructuredPairs) > 0 {
			workingPairs = append(workingPairs, newUnstructuredPairs...)
			klog.V(2).Infof("Added %d network pairs to mapping '%s'", len(newUnstructuredPairs), name)
		} else {
			klog.V(2).Infof("No new network pairs to add after filtering duplicates")
		}
	}

	// Process updates
	if updatePairs != "" {
		klog.V(2).Infof("Updating network pairs in mapping: %s", updatePairs)
		updatePairsList, err := mapping.ParseNetworkPairs(updatePairs, sourceProviderNamespace, configFlags, sourceProviderName, inventoryURL)
		if err != nil {
			return fmt.Errorf("failed to parse update-pairs: %v", err)
		}

		// Convert update pairs to unstructured format
		var updateUnstructuredPairs []interface{}
		for _, pair := range updatePairsList {
			pairMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pair)
			if err != nil {
				klog.V(2).Infof("Warning: Failed to convert update pair to unstructured, skipping: %v", err)
				continue
			}
			updateUnstructuredPairs = append(updateUnstructuredPairs, pairMap)
		}

		workingPairs = updateUnstructuredPairsBySource(workingPairs, updateUnstructuredPairs)
		klog.V(2).Infof("Updated %d network pairs in mapping '%s'", len(updateUnstructuredPairs), name)
	}

	klog.V(3).Infof("Final working pairs count: %d", len(workingPairs))

	// Patch the spec.map field (workingPairs is already unstructured)
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"map": workingPairs,
		},
	}

	patchBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, &unstructured.Unstructured{Object: patchData})
	if err != nil {
		return fmt.Errorf("failed to encode patch data: %v", err)
	}

	// Apply the patch
	_, err = dynamicClient.Resource(client.NetworkMapGVR).Namespace(namespace).Patch(
		context.TODO(),
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch network mapping: %v", err)
	}

	fmt.Printf("networkmap/%s patched\n", name)
	return nil
}

// removeSourceFromUnstructuredPairs removes pairs with matching source names/IDs from unstructured pairs
func removeSourceFromUnstructuredPairs(pairs []interface{}, sourcesToRemove []string) []interface{} {
	var filteredPairs []interface{}

	for _, pairInterface := range pairs {
		pairMap, ok := pairInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract source information
		sourceInterface, found := pairMap["source"]
		if !found {
			filteredPairs = append(filteredPairs, pairInterface)
			continue
		}

		sourceMap, ok := sourceInterface.(map[string]interface{})
		if !ok {
			filteredPairs = append(filteredPairs, pairInterface)
			continue
		}

		sourceName, _ := sourceMap["name"].(string)
		sourceID, _ := sourceMap["id"].(string)

		shouldRemove := false
		for _, sourceToRemove := range sourcesToRemove {
			if sourceName == sourceToRemove || sourceID == sourceToRemove {
				shouldRemove = true
				break
			}
		}

		if !shouldRemove {
			filteredPairs = append(filteredPairs, pairInterface)
		}
	}

	return filteredPairs
}

// updateUnstructuredPairsBySource replaces all existing pairs for each updated
// source with the new pair(s). A single source may map to multiple destinations
// (1:N), so --update-pairs src:nad1,src:nad2 replaces every old mapping for
// "src" with the two new ones. Sources not mentioned in newPairs are kept as-is.
func updateUnstructuredPairsBySource(existingPairs []interface{}, newPairs []interface{}) []interface{} {
	// Collect the set of source name/IDs being updated
	updatedSources := make(map[string]bool)
	for _, p := range newPairs {
		pairMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		srcIface, ok := pairMap["source"]
		if !ok {
			continue
		}
		srcMap, ok := srcIface.(map[string]interface{})
		if !ok {
			continue
		}
		if name, _ := srcMap["name"].(string); name != "" {
			updatedSources[name] = true
		}
		if id, _ := srcMap["id"].(string); id != "" {
			updatedSources[id] = true
		}
	}

	// Keep existing pairs whose source is NOT being replaced
	var result []interface{}
	for _, p := range existingPairs {
		pairMap, ok := p.(map[string]interface{})
		if !ok {
			result = append(result, p)
			continue
		}
		srcIface, ok := pairMap["source"]
		if !ok {
			result = append(result, p)
			continue
		}
		srcMap, ok := srcIface.(map[string]interface{})
		if !ok {
			result = append(result, p)
			continue
		}
		name, _ := srcMap["name"].(string)
		id, _ := srcMap["id"].(string)
		if (name != "" && updatedSources[name]) || (id != "" && updatedSources[id]) {
			continue // will be replaced by newPairs
		}
		result = append(result, p)
	}

	// Append all new pairs
	result = append(result, newPairs...)
	return result
}
