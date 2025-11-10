package mapping

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// removeSourceFromUnstructuredStoragePairs removes pairs with matching source names/IDs from unstructured pairs
func removeSourceFromUnstructuredStoragePairs(pairs []interface{}, sourcesToRemove []string) []interface{} {
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

// checkUnstructuredStorageSourceDuplicates checks if any of the new pairs have sources that already exist in current pairs
func checkUnstructuredStorageSourceDuplicates(currentPairs []interface{}, newPairs []interface{}) []string {
	var duplicates []string

	// Create a map of existing sources for quick lookup
	existingSourceMap := make(map[string]bool)
	for _, pairInterface := range currentPairs {
		pairMap, ok := pairInterface.(map[string]interface{})
		if !ok {
			continue
		}

		sourceInterface, found := pairMap["source"]
		if !found {
			continue
		}

		sourceMap, ok := sourceInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if sourceName, ok := sourceMap["name"].(string); ok && sourceName != "" {
			existingSourceMap[sourceName] = true
		}
		if sourceID, ok := sourceMap["id"].(string); ok && sourceID != "" {
			existingSourceMap[sourceID] = true
		}
	}

	// Check new pairs against existing sources
	for _, pairInterface := range newPairs {
		pairMap, ok := pairInterface.(map[string]interface{})
		if !ok {
			continue
		}

		sourceInterface, found := pairMap["source"]
		if !found {
			continue
		}

		sourceMap, ok := sourceInterface.(map[string]interface{})
		if !ok {
			continue
		}

		sourceName, _ := sourceMap["name"].(string)
		sourceID, _ := sourceMap["id"].(string)

		if sourceName != "" && existingSourceMap[sourceName] {
			duplicates = append(duplicates, sourceName)
		} else if sourceID != "" && existingSourceMap[sourceID] {
			duplicates = append(duplicates, sourceID)
		}
	}

	return duplicates
}

// filterOutDuplicateUnstructuredStoragePairs removes pairs that have duplicate sources, keeping only unique ones
func filterOutDuplicateUnstructuredStoragePairs(currentPairs []interface{}, newPairs []interface{}) []interface{} {
	// Create a map of existing sources for quick lookup
	existingSourceMap := make(map[string]bool)
	for _, pairInterface := range currentPairs {
		pairMap, ok := pairInterface.(map[string]interface{})
		if !ok {
			continue
		}

		sourceInterface, found := pairMap["source"]
		if !found {
			continue
		}

		sourceMap, ok := sourceInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if sourceName, ok := sourceMap["name"].(string); ok && sourceName != "" {
			existingSourceMap[sourceName] = true
		}
		if sourceID, ok := sourceMap["id"].(string); ok && sourceID != "" {
			existingSourceMap[sourceID] = true
		}
	}

	// Filter new pairs to exclude duplicates
	var filteredPairs []interface{}
	for _, pairInterface := range newPairs {
		pairMap, ok := pairInterface.(map[string]interface{})
		if !ok {
			continue
		}

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

		isDuplicate := false
		if sourceName != "" && existingSourceMap[sourceName] {
			isDuplicate = true
		} else if sourceID != "" && existingSourceMap[sourceID] {
			isDuplicate = true
		}

		if !isDuplicate {
			filteredPairs = append(filteredPairs, pairInterface)
		}
	}

	return filteredPairs
}

// updateUnstructuredStoragePairsBySource updates or adds pairs based on source name/ID matching
func updateUnstructuredStoragePairsBySource(existingPairs []interface{}, newPairs []interface{}) []interface{} {
	updatedPairs := make([]interface{}, len(existingPairs))
	copy(updatedPairs, existingPairs)

	for _, newPairInterface := range newPairs {
		newPairMap, ok := newPairInterface.(map[string]interface{})
		if !ok {
			continue
		}

		newSourceInterface, found := newPairMap["source"]
		if !found {
			// Add new pair if no source info
			updatedPairs = append(updatedPairs, newPairInterface)
			continue
		}

		newSourceMap, ok := newSourceInterface.(map[string]interface{})
		if !ok {
			updatedPairs = append(updatedPairs, newPairInterface)
			continue
		}

		newSourceName, _ := newSourceMap["name"].(string)
		newSourceID, _ := newSourceMap["id"].(string)

		found = false
		for i, existingPairInterface := range updatedPairs {
			existingPairMap, ok := existingPairInterface.(map[string]interface{})
			if !ok {
				continue
			}

			existingSourceInterface, hasSource := existingPairMap["source"]
			if !hasSource {
				continue
			}

			existingSourceMap, ok := existingSourceInterface.(map[string]interface{})
			if !ok {
				continue
			}

			existingSourceName, _ := existingSourceMap["name"].(string)
			existingSourceID, _ := existingSourceMap["id"].(string)

			if (existingSourceName != "" && existingSourceName == newSourceName) ||
				(existingSourceID != "" && existingSourceID == newSourceID) {
				// Update existing pair
				updatedPairs[i] = newPairInterface
				found = true
				break
			}
		}
		if !found {
			// Add new pair
			updatedPairs = append(updatedPairs, newPairInterface)
		}
	}

	return updatedPairs
}

// patchStorageMappingWithOptions patches an existing storage mapping with additional options for VolumeMode, AccessMode, and OffloadPlugin
func patchStorageMappingWithOptions(configFlags *genericclioptions.ConfigFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL string, defaultVolumeMode, defaultAccessMode, defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor string) error {
	klog.V(2).Infof("Patching storage mapping '%s' in namespace '%s' with enhanced options", name, namespace)

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the existing mapping
	existingMapping, err := dynamicClient.Resource(client.StorageMapGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get storage mapping '%s': %v", name, err)
	}

	// Extract source provider for pair resolution
	sourceProviderName, sourceProviderNamespace, err := getSourceProviderFromMapping(existingMapping)
	if err != nil {
		return fmt.Errorf("failed to get source provider from mapping: %v", err)
	}

	if sourceProviderNamespace != "" {
		klog.V(2).Infof("Using source provider '%s/%s' for storage pair resolution", sourceProviderNamespace, sourceProviderName)
	} else {
		klog.V(2).Infof("Using source provider '%s' for storage pair resolution", sourceProviderName)
	}

	// Extract the existing storage pairs from the unstructured mapping
	// Work with unstructured data throughout to avoid reflection issues with Referenced field
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
	klog.V(3).Infof("Current mapping has %d storage pairs", len(workingPairs))

	// Process removals first
	if removePairs != "" {
		sourcesToRemove := parseSourcesToRemove(removePairs)
		klog.V(2).Infof("Removing %d storage pairs from mapping", len(sourcesToRemove))
		workingPairs = removeSourceFromUnstructuredStoragePairs(workingPairs, sourcesToRemove)
		klog.V(2).Infof("Successfully removed storage pairs from mapping '%s'", name)
	}

	// Process additions
	if addPairs != "" {
		klog.V(2).Infof("Adding storage pairs to mapping: %s", addPairs)
		newPairs, err := mapping.ParseStoragePairsWithOptions(mapping.StorageParseOptions{
			PairStr:              addPairs,
			DefaultNamespace:     sourceProviderNamespace,
			ConfigFlags:          configFlags,
			SourceProvider:       sourceProviderName,
			InventoryURL:         inventoryURL,
			DefaultVolumeMode:    defaultVolumeMode,
			DefaultAccessMode:    defaultAccessMode,
			DefaultOffloadPlugin: defaultOffloadPlugin,
			DefaultOffloadSecret: defaultOffloadSecret,
			DefaultOffloadVendor: defaultOffloadVendor,
		})
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

		// Check for duplicate sources
		duplicates := checkUnstructuredStorageSourceDuplicates(workingPairs, newUnstructuredPairs)
		if len(duplicates) > 0 {
			klog.V(1).Infof("Warning: Found duplicate sources in add-pairs, skipping: %v", duplicates)
			fmt.Printf("Warning: Skipping duplicate sources: %s\n", strings.Join(duplicates, ", "))
			newUnstructuredPairs = filterOutDuplicateUnstructuredStoragePairs(workingPairs, newUnstructuredPairs)
		}

		if len(newUnstructuredPairs) > 0 {
			workingPairs = append(workingPairs, newUnstructuredPairs...)
			klog.V(2).Infof("Added %d storage pairs to mapping '%s'", len(newUnstructuredPairs), name)
		} else {
			klog.V(2).Infof("No new storage pairs to add after filtering duplicates")
		}
	}

	// Process updates
	if updatePairs != "" {
		klog.V(2).Infof("Updating storage pairs in mapping: %s", updatePairs)
		updatePairsList, err := mapping.ParseStoragePairsWithOptions(mapping.StorageParseOptions{
			PairStr:              updatePairs,
			DefaultNamespace:     sourceProviderNamespace,
			ConfigFlags:          configFlags,
			SourceProvider:       sourceProviderName,
			InventoryURL:         inventoryURL,
			DefaultVolumeMode:    defaultVolumeMode,
			DefaultAccessMode:    defaultAccessMode,
			DefaultOffloadPlugin: defaultOffloadPlugin,
			DefaultOffloadSecret: defaultOffloadSecret,
			DefaultOffloadVendor: defaultOffloadVendor,
		})
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

		workingPairs = updateUnstructuredStoragePairsBySource(workingPairs, updateUnstructuredPairs)
		klog.V(2).Infof("Updated %d storage pairs in mapping '%s'", len(updateUnstructuredPairs), name)
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
	_, err = dynamicClient.Resource(client.StorageMapGVR).Namespace(namespace).Patch(
		context.TODO(),
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch storage mapping: %v", err)
	}

	fmt.Printf("storagemap/%s patched\n", name)
	return nil
}
