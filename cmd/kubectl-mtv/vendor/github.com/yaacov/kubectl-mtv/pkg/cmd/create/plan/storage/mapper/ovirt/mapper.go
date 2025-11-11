package ovirt

import (
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
)

// OvirtStorageMapper implements storage mapping for oVirt providers
type OvirtStorageMapper struct{}

// NewOvirtStorageMapper creates a new oVirt storage mapper
func NewOvirtStorageMapper() mapper.StorageMapper {
	return &OvirtStorageMapper{}
}

// CreateStoragePairs creates storage mapping pairs using generic logic (no same-name matching)
func (m *OvirtStorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	klog.V(4).Infof("DEBUG: oVirt storage mapper - Creating storage pairs - %d source storages, %d target storages", len(sourceStorages), len(targetStorages))

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("DEBUG: No source storages to map")
		return storagePairs, nil
	}

	// Use generic default behavior (all sources -> single default target)
	defaultStorageClass := findDefaultStorageClass(targetStorages, opts)
	klog.V(4).Infof("DEBUG: Selected default storage class: %s", defaultStorageClass.StorageClass)

	// Map all source storages to the default storage class
	for _, sourceStorage := range sourceStorages {
		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source:      sourceStorage,
			Destination: defaultStorageClass,
		})
		klog.V(4).Infof("DEBUG: Mapped source storage %s -> %s", sourceStorage.Name, defaultStorageClass.StorageClass)
	}

	klog.V(4).Infof("DEBUG: Created %d storage pairs", len(storagePairs))
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
