package mapping

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// FindStorageClass looks up target storage class for Azure disk SKU (Premium_LRS, Standard_LRS, etc).
func FindStorageClass(storageMap *api.StorageMap, diskSKU string) string {
	if storageMap == nil {
		return ""
	}

	for _, mapping := range storageMap.Spec.Map {
		if mapping.Source.Name == diskSKU {
			return mapping.Destination.StorageClass
		}
	}

	return ""
}

// HasStorageMapping checks if a storage mapping exists for the given disk SKU.
func HasStorageMapping(storageMap *api.StorageMap, diskSKU string) bool {
	return FindStorageClass(storageMap, diskSKU) != ""
}

// FindNetworkPair finds the network mapping for a given subnet ID.
func FindNetworkPair(networkMap *api.NetworkMap, subnetID string) *api.NetworkPair {
	if networkMap == nil || networkMap.Spec.Map == nil {
		return nil
	}

	for i := range networkMap.Spec.Map {
		candidate := &networkMap.Spec.Map[i]
		if candidate.Source.ID == subnetID || candidate.Source.Name == subnetID {
			return candidate
		}
	}

	return nil
}

// HasNetworkMapping checks if a network mapping exists for the given subnet ID.
func HasNetworkMapping(networkMap *api.NetworkMap, subnetID string) bool {
	return FindNetworkPair(networkMap, subnetID) != nil
}
