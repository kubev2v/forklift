package openshift

import (
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
)

// OpenShiftStorageMapper implements storage mapping for OpenShift providers
type OpenShiftStorageMapper struct{}

// NewOpenShiftStorageMapper creates a new OpenShift storage mapper
func NewOpenShiftStorageMapper() mapper.StorageMapper {
	return &OpenShiftStorageMapper{}
}

// CreateStoragePairs creates storage mapping pairs with OpenShift-specific logic
func (m *OpenShiftStorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	klog.V(4).Infof("DEBUG: OpenShift storage mapper - Creating storage pairs - %d source storages, %d target storages", len(sourceStorages), len(targetStorages))
	klog.V(4).Infof("DEBUG: Source provider type: %s, Target provider type: %s", opts.SourceProviderType, opts.TargetProviderType)

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("DEBUG: No source storages to map")
		return storagePairs, nil
	}

	// For OCP-to-OCP: Try same-name matching (all-or-nothing)
	if opts.TargetProviderType == "openshift" {
		klog.V(4).Infof("DEBUG: OCP-to-OCP migration detected, attempting same-name matching")
		if canMatchAllStoragesByName(sourceStorages, targetStorages) {
			klog.V(4).Infof("DEBUG: All storages can be matched by name, using same-name mapping")
			return createSameNameStoragePairs(sourceStorages, targetStorages)
		}
		klog.V(4).Infof("DEBUG: Not all storages can be matched by name, falling back to default behavior")
	}

	// Fall back to default behavior
	return createDefaultStoragePairs(sourceStorages, targetStorages, opts)
}

// canMatchAllStoragesByName checks if every source storage has a matching target storage by name
func canMatchAllStoragesByName(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage) bool {
	// Create a map of target storage class names for quick lookup
	targetNames := make(map[string]bool)
	for _, target := range targetStorages {
		if target.StorageClass != "" {
			targetNames[target.StorageClass] = true
		}
	}

	klog.V(4).Infof("DEBUG: Available target storage classes: %v", getTargetStorageNames(targetStorages))

	// Check if every source has a matching target by name
	for _, source := range sourceStorages {
		if !targetNames[source.Name] {
			klog.V(4).Infof("DEBUG: Source storage '%s' has no matching target by name", source.Name)
			return false
		}
	}

	klog.V(4).Infof("DEBUG: All source storages can be matched by name")
	return true
}

// createSameNameStoragePairs creates storage pairs using same-name matching
func createSameNameStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	// Create a map of target storages by name for quick lookup
	targetByName := make(map[string]forkliftv1beta1.DestinationStorage)
	for _, target := range targetStorages {
		if target.StorageClass != "" {
			targetByName[target.StorageClass] = target
		}
	}

	// Create pairs using same-name matching
	for _, sourceStorage := range sourceStorages {
		if targetStorage, exists := targetByName[sourceStorage.Name]; exists {
			storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
				Source:      sourceStorage,
				Destination: targetStorage,
			})
			klog.V(4).Infof("DEBUG: Mapped source storage %s -> %s (same name)", sourceStorage.Name, targetStorage.StorageClass)
		}
	}

	klog.V(4).Infof("DEBUG: Created %d same-name storage pairs", len(storagePairs))
	return storagePairs, nil
}

// createDefaultStoragePairs creates storage pairs using the default behavior (all sources -> single default target)
func createDefaultStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	// Find default storage class using the same logic as the original mapper
	defaultStorageClass := findDefaultStorageClass(targetStorages, opts)
	klog.V(4).Infof("DEBUG: Selected default storage class: %s", defaultStorageClass.StorageClass)

	// Map all source storages to the default storage class
	for _, sourceStorage := range sourceStorages {
		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source:      sourceStorage,
			Destination: defaultStorageClass,
		})
		klog.V(4).Infof("DEBUG: Mapped source storage %s -> %s (default)", sourceStorage.Name, defaultStorageClass.StorageClass)
	}

	klog.V(4).Infof("DEBUG: Created %d default storage pairs", len(storagePairs))
	return storagePairs, nil
}

// findDefaultStorageClass finds the default storage class using the original priority logic
func findDefaultStorageClass(targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) forkliftv1beta1.DestinationStorage {
	// Priority 1: If user explicitly specified a default storage class, use it
	if opts.DefaultTargetStorageClass != "" {
		defaultStorage := forkliftv1beta1.DestinationStorage{
			StorageClass: opts.DefaultTargetStorageClass,
		}
		klog.V(4).Infof("DEBUG: Using user-defined default storage class: %s", opts.DefaultTargetStorageClass)
		return defaultStorage
	}

	// Priority 2-5: Use the target storage selected by FetchTargetStorages
	// (which implements: virt annotation -> k8s annotation -> name with "virtualization" -> first available)
	if len(targetStorages) > 0 {
		defaultStorage := targetStorages[0]
		klog.V(4).Infof("DEBUG: Using auto-selected storage class: %s", defaultStorage.StorageClass)
		return defaultStorage
	}

	// Priority 6: Fall back to empty storage class (system default)
	klog.V(4).Infof("DEBUG: No storage classes available, using system default")
	return forkliftv1beta1.DestinationStorage{}
}

// getTargetStorageNames returns a slice of target storage class names for logging
func getTargetStorageNames(targetStorages []forkliftv1beta1.DestinationStorage) []string {
	var names []string
	for _, target := range targetStorages {
		if target.StorageClass != "" {
			names = append(names, target.StorageClass)
		}
	}
	return names
}
