package mapping

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// PatchNetwork patches a network mapping
func PatchNetwork(configFlags *genericclioptions.ConfigFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL string) error {
	return patchNetworkMapping(configFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL)
}

// PatchStorage patches a storage mapping (wrapper for backward compatibility)
func PatchStorage(configFlags *genericclioptions.ConfigFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL string) error {
	return PatchStorageWithOptions(configFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL, "", "", "", "", "")
}

// PatchStorageWithOptions patches a storage mapping with additional options for VolumeMode, AccessMode, and OffloadPlugin
func PatchStorageWithOptions(configFlags *genericclioptions.ConfigFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL string, defaultVolumeMode, defaultAccessMode, defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor string) error {
	return patchStorageMappingWithOptions(configFlags, name, namespace, addPairs, updatePairs, removePairs, inventoryURL, defaultVolumeMode, defaultAccessMode, defaultOffloadPlugin, defaultOffloadSecret, defaultOffloadVendor)
}

// getSourceProviderFromMapping extracts the source provider name and namespace from a mapping
func getSourceProviderFromMapping(mapping *unstructured.Unstructured) (string, string, error) {
	provider, found, err := unstructured.NestedMap(mapping.Object, "spec", "provider", "source")
	if err != nil {
		return "", "", fmt.Errorf("failed to get source provider: %v", err)
	}
	if !found || provider == nil {
		return "", "", fmt.Errorf("source provider not found in mapping")
	}

	name, nameOk := provider["name"].(string)
	namespace, namespaceOk := provider["namespace"].(string)

	if !nameOk {
		return "", "", fmt.Errorf("source provider name not found")
	}

	// namespace is optional, so we don't error if it's not found
	if !namespaceOk {
		namespace = ""
	}

	return name, namespace, nil
}

// parseSourcesToRemove parses a comma-separated list of source names to remove
func parseSourcesToRemove(removeStr string) []string {
	if removeStr == "" {
		return nil
	}

	var sources []string
	sourceList := strings.Split(removeStr, ",")

	for _, source := range sourceList {
		source = strings.TrimSpace(source)
		if source != "" {
			sources = append(sources, source)
		}
	}

	return sources
}
