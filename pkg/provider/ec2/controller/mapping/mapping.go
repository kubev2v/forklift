// Package mapping provides shared utilities for network and storage mapping lookups.
// Used by both builder and validator packages to avoid code duplication.
package mapping

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// FindStorageClass looks up target storage class for EBS volume type (gp2, gp3, io1, etc).
// Returns storage class name from StorageMap or empty string if no mapping found.
func FindStorageClass(storageMap *api.StorageMap, volumeType string) string {
	if storageMap == nil {
		return ""
	}

	for _, mapping := range storageMap.Spec.Map {
		if mapping.Source.Name == volumeType {
			return mapping.Destination.StorageClass
		}
	}

	return ""
}

// HasStorageMapping checks if a storage mapping exists for the given volume type.
func HasStorageMapping(storageMap *api.StorageMap, volumeType string) bool {
	return FindStorageClass(storageMap, volumeType) != ""
}

// FindNetworkPair finds the network mapping for a given subnet ID.
// Matches by Source.ID or Source.Name (both can contain the subnet ID).
// Returns the matching NetworkPair or nil if no mapping found.
func FindNetworkPair(networkMap *api.NetworkMap, subnetID string) *api.NetworkPair {
	if networkMap == nil || networkMap.Spec.Map == nil {
		return nil
	}

	for i := range networkMap.Spec.Map {
		candidate := &networkMap.Spec.Map[i]
		// Match by subnet ID in either ID or Name field
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
