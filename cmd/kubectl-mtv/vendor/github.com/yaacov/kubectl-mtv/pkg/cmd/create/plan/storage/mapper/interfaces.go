package mapper

import (
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

// StorageMappingOptions contains options for storage mapping
type StorageMappingOptions struct {
	DefaultTargetStorageClass string
	SourceProviderType        string
	TargetProviderType        string
}

// StorageMapper defines the interface for storage mapping operations
type StorageMapper interface {
	CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error)
}
