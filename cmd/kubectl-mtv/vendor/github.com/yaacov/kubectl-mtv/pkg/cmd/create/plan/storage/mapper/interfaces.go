package mapper

import (
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	corev1 "k8s.io/api/core/v1"
)

// StorageMappingOptions contains options for storage mapping
type StorageMappingOptions struct {
	DefaultTargetStorageClass string
	SourceProviderType        string
	TargetProviderType        string
	DefaultVolumeMode         string
	DefaultAccessMode         string
	DefaultOffloadPlugin      string
	DefaultOffloadSecret      string
	DefaultOffloadVendor      string
}

// StorageMapper defines the interface for storage mapping operations
type StorageMapper interface {
	CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error)
}

// ApplyOffloadToPairs applies default volume mode, access mode, and offload
// plugin settings from opts to every pair that does not already have them set.
func ApplyOffloadToPairs(pairs []forkliftv1beta1.StoragePair, opts StorageMappingOptions) []forkliftv1beta1.StoragePair {
	for i := range pairs {
		if opts.DefaultVolumeMode != "" && pairs[i].Destination.VolumeMode == "" {
			pairs[i].Destination.VolumeMode = corev1.PersistentVolumeMode(opts.DefaultVolumeMode)
		}

		if opts.DefaultAccessMode != "" && pairs[i].Destination.AccessMode == "" {
			pairs[i].Destination.AccessMode = corev1.PersistentVolumeAccessMode(opts.DefaultAccessMode)
		}

		if opts.DefaultOffloadPlugin != "" && opts.DefaultOffloadVendor != "" && pairs[i].OffloadPlugin == nil {
			switch opts.DefaultOffloadPlugin {
			case "vsphere":
				pairs[i].OffloadPlugin = &forkliftv1beta1.OffloadPlugin{
					VSphereXcopyPluginConfig: &forkliftv1beta1.VSphereXcopyPluginConfig{
						SecretRef:            opts.DefaultOffloadSecret,
						StorageVendorProduct: forkliftv1beta1.StorageVendorProduct(opts.DefaultOffloadVendor),
					},
				}
			}
		}
	}

	return pairs
}
