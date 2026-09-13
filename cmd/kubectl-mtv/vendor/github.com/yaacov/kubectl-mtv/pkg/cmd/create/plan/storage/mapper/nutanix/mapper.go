package nutanix

import (
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
)

// StorageMapper implements storage mapping for Nutanix providers.
type StorageMapper struct{}

// NewStorageMapper creates a Nutanix storage mapper.
func NewStorageMapper() mapper.StorageMapper {
	return &StorageMapper{}
}

// CreateStoragePairs maps all source storage containers to a single default storage class.
func (m *StorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	storagePairs := make([]forkliftv1beta1.StoragePair, 0, len(sourceStorages))
	if len(sourceStorages) == 0 {
		return storagePairs, nil
	}

	defaultStorageClass := findDefaultStorageClass(targetStorages, opts)
	for _, sourceStorage := range sourceStorages {
		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source:      sourceStorage,
			Destination: defaultStorageClass,
		})
	}
	return mapper.ApplyOffloadToPairs(storagePairs, opts), nil
}

func findDefaultStorageClass(targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) forkliftv1beta1.DestinationStorage {
	if opts.DefaultTargetStorageClass != "" {
		return forkliftv1beta1.DestinationStorage{StorageClass: opts.DefaultTargetStorageClass}
	}
	if len(targetStorages) > 0 {
		return targetStorages[0]
	}
	return forkliftv1beta1.DestinationStorage{}
}
