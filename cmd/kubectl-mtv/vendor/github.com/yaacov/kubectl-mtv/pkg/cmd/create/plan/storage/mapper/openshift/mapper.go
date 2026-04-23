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

// CreateStoragePairs creates storage mapping pairs with OpenShift-specific logic.
//
// Three-step flow:
//
//	(a) If the user specified --default-target-storage-class, map every source to that SC.
//	(b) For OCP-to-OCP, try same-name matching per source against ALL target SCs.
//	(c) Any source that didn't get a same-name match is mapped to the default SC (targetStorages[0]).
func (m *OpenShiftStorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	klog.V(4).Infof("DEBUG: OpenShift storage mapper - Creating storage pairs - %d source storages, %d target storages", len(sourceStorages), len(targetStorages))
	klog.V(4).Infof("DEBUG: Source provider type: %s, Target provider type: %s", opts.SourceProviderType, opts.TargetProviderType)

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("DEBUG: No source storages to map")
		return storagePairs, nil
	}

	// (a) User specified a default SC — map every source to it.
	if opts.DefaultTargetStorageClass != "" {
		return createDefaultStoragePairs(sourceStorages, opts.DefaultTargetStorageClass)
	}

	// Resolve the default SC for gap-filling (best SC selected by the target fetcher).
	defaultSC := findDefaultStorageClass(targetStorages)

	// (b) + (c) For OCP-to-OCP: same-name matching with gap-fill.
	if opts.TargetProviderType == "openshift" {
		klog.V(4).Infof("DEBUG: OCP-to-OCP migration detected, attempting same-name matching with gap-fill")
		return createSameNameWithFallback(sourceStorages, targetStorages, defaultSC)
	}

	// Non-OCP target: map all sources to the default SC.
	return createAllToDefaultPairs(sourceStorages, defaultSC)
}

// createDefaultStoragePairs maps every source to the user-specified SC.
func createDefaultStoragePairs(sourceStorages []ref.Ref, storageClass string) ([]forkliftv1beta1.StoragePair, error) {
	klog.V(4).Infof("DEBUG: Using user-defined default storage class: %s", storageClass)
	storagePairs := make([]forkliftv1beta1.StoragePair, 0, len(sourceStorages))
	dest := forkliftv1beta1.DestinationStorage{StorageClass: storageClass}
	for _, src := range sourceStorages {
		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source:      src,
			Destination: dest,
		})
		klog.V(4).Infof("DEBUG: Mapped source storage %s -> %s (user default)", src.Name, storageClass)
	}
	return storagePairs, nil
}

// createSameNameWithFallback matches sources to targets by name, then fills
// any gaps with the default SC.
func createSameNameWithFallback(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, defaultSC forkliftv1beta1.DestinationStorage) ([]forkliftv1beta1.StoragePair, error) {
	storagePairs := make([]forkliftv1beta1.StoragePair, 0, len(sourceStorages))

	targetByName := make(map[string]forkliftv1beta1.DestinationStorage, len(targetStorages))
	for _, target := range targetStorages {
		if target.StorageClass != "" {
			targetByName[target.StorageClass] = target
		}
	}

	klog.V(4).Infof("DEBUG: Available target storage classes: %v", getTargetStorageNames(targetStorages))

	for _, src := range sourceStorages {
		if target, exists := targetByName[src.Name]; exists {
			storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
				Source:      src,
				Destination: target,
			})
			klog.V(4).Infof("DEBUG: Mapped source storage %s -> %s (same name)", src.Name, target.StorageClass)
		} else {
			storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
				Source:      src,
				Destination: defaultSC,
			})
			klog.V(2).Infof("WARNING: OpenShift storage mapper - No same-name match for '%s', using default SC '%s'", src.Name, defaultSC.StorageClass)
		}
	}

	klog.V(4).Infof("DEBUG: Created %d storage pairs (same-name with fallback)", len(storagePairs))
	return storagePairs, nil
}

// createAllToDefaultPairs maps every source to the default SC.
func createAllToDefaultPairs(sourceStorages []ref.Ref, defaultSC forkliftv1beta1.DestinationStorage) ([]forkliftv1beta1.StoragePair, error) {
	storagePairs := make([]forkliftv1beta1.StoragePair, 0, len(sourceStorages))
	for _, src := range sourceStorages {
		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source:      src,
			Destination: defaultSC,
		})
		klog.V(4).Infof("DEBUG: Mapped source storage %s -> %s (default)", src.Name, defaultSC.StorageClass)
	}
	klog.V(4).Infof("DEBUG: Created %d default storage pairs", len(storagePairs))
	return storagePairs, nil
}

// findDefaultStorageClass returns the best target SC. The OpenShift fetcher
// places the highest-priority SC at index 0 (virt annotation > k8s annotation
// > name with "virtualization" > first available).
func findDefaultStorageClass(targetStorages []forkliftv1beta1.DestinationStorage) forkliftv1beta1.DestinationStorage {
	if len(targetStorages) > 0 {
		klog.V(4).Infof("DEBUG: Using auto-selected storage class: %s", targetStorages[0].StorageClass)
		return targetStorages[0]
	}

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
