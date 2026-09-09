package azure

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
)

const AzureCSIDriver = "disk.csi.azure.com"

// AzureStorageMapper implements storage mapping for Azure providers
type AzureStorageMapper struct{}

// NewAzureStorageMapper creates a new Azure storage mapper
func NewAzureStorageMapper() mapper.StorageMapper {
	return &AzureStorageMapper{}
}

// getDiskTypeConfig returns the volume mode and access mode for an Azure disk type SKU
func getDiskTypeConfig(diskType string) (corev1.PersistentVolumeMode, corev1.PersistentVolumeAccessMode) {
	switch strings.ToLower(diskType) {
	case "premium_lrs", "premium_zrs", "premiumv2_lrs", "ultrassd_lrs":
		return corev1.PersistentVolumeBlock, corev1.ReadWriteOnce
	case "standard_lrs", "standardssd_lrs", "standardssd_zrs":
		return corev1.PersistentVolumeBlock, corev1.ReadWriteOnce
	default:
		return corev1.PersistentVolumeBlock, corev1.ReadWriteOnce
	}
}

// findAzureCSIStorageClass finds the first target SC backed by disk.csi.azure.com.
func findAzureCSIStorageClass(targetStorages []forkliftv1beta1.DestinationStorage, provisioners map[string]string) string {
	if len(provisioners) == 0 {
		return ""
	}
	for _, storage := range targetStorages {
		if provisioners[storage.StorageClass] == AzureCSIDriver {
			return storage.StorageClass
		}
	}
	return ""
}

// findMatchingAzureStorageClass tries to find a target SC that matches the given
// Azure disk type by name, preferring Azure CSI-backed SCs.
func findMatchingAzureStorageClass(diskType string, targetStorages []forkliftv1beta1.DestinationStorage, provisioners map[string]string) string {
	if len(targetStorages) == 0 || diskType == "" {
		return ""
	}

	diskTypeLower := strings.ToLower(diskType)
	diskTypeNormalized := strings.ReplaceAll(diskTypeLower, "_", "-")

	// First pass: find name matches that are also Azure CSI-backed
	for _, storage := range targetStorages {
		scName := strings.ToLower(storage.StorageClass)
		if scName == "" {
			continue
		}
		if !nameMatches(scName, diskTypeLower, diskTypeNormalized) {
			continue
		}
		if provisioners[storage.StorageClass] == AzureCSIDriver {
			klog.V(4).Infof("Azure storage mapper - Found CSI name match: %s -> %s", diskType, storage.StorageClass)
			return storage.StorageClass
		}
	}

	// Second pass: any name match, but only if it's also Azure CSI-backed
	for _, storage := range targetStorages {
		scName := strings.ToLower(storage.StorageClass)
		if scName == "" {
			continue
		}
		if nameMatches(scName, diskTypeLower, diskTypeNormalized) {
			klog.V(4).Infof("Azure storage mapper - Found name match (non-CSI): %s -> %s, skipping", diskType, storage.StorageClass)
		}
	}

	klog.V(4).Infof("Azure storage mapper - No name match for %s", diskType)
	return ""
}

func nameMatches(scName, diskTypeLower, diskTypeNormalized string) bool {
	if scName == diskTypeLower {
		return true
	}
	if strings.Contains(scName, diskTypeLower) {
		return true
	}
	if strings.Contains(scName, diskTypeNormalized) {
		return true
	}
	return false
}

// CreateStoragePairs creates storage mapping pairs for Azure -> OpenShift migrations.
func (m *AzureStorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	if opts.TargetProviderType != "" && opts.TargetProviderType != "openshift" {
		klog.V(2).Infof("WARNING: Azure storage mapper - Target provider type is '%s', not 'openshift'. Azure->%s migrations may not work as expected.",
			opts.TargetProviderType, opts.TargetProviderType)
	}

	klog.V(4).Infof("Azure storage mapper - Creating storage pairs for %d source disk types", len(sourceStorages))

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("No source storages to map")
		return storagePairs, nil
	}

	provisioners := opts.TargetStorageProvisioners

	// User specified a default SC -- validate and map every source to it
	if opts.DefaultTargetStorageClass != "" {
		klog.V(4).Infof("Azure storage mapper - Using user-defined storage class '%s' for all types", opts.DefaultTargetStorageClass)

		if err := validateAzureCSIProvisioner(opts.DefaultTargetStorageClass, provisioners); err != nil {
			return nil, err
		}

		for _, sourceStorage := range sourceStorages {
			volumeMode, accessMode := getDiskTypeConfig(sourceStorage.Name)
			storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
				Source: sourceStorage,
				Destination: forkliftv1beta1.DestinationStorage{
					StorageClass: opts.DefaultTargetStorageClass,
					VolumeMode:   volumeMode,
					AccessMode:   accessMode,
				},
			})
		}
		klog.V(4).Infof("Azure storage mapper - Created %d storage pairs (user default)", len(storagePairs))

		storagePairs = mapper.ApplyOffloadToPairs(storagePairs, opts)
		return storagePairs, nil
	}

	// Auto-mapping: find the best default SC (must be Azure CSI-backed)
	defaultSC := findAzureCSIStorageClass(targetStorages, provisioners)
	if defaultSC == "" {
		return nil, fmt.Errorf(
			"no StorageClass with provisioner '%s' found on the target cluster; "+
				"Azure migrations require an Azure CSI-backed StorageClass (e.g. 'managed-csi'). "+
				"Use --default-target-storage-class to specify one explicitly, or install the Azure CSI driver",
			AzureCSIDriver)
	}
	klog.V(4).Infof("Azure storage mapper - Selected Azure CSI default SC: %s", defaultSC)

	for _, sourceStorage := range sourceStorages {
		diskType := sourceStorage.Name
		volumeMode, accessMode := getDiskTypeConfig(diskType)

		ocpStorageClass := findMatchingAzureStorageClass(diskType, targetStorages, provisioners)
		if ocpStorageClass != "" {
			klog.V(4).Infof("Azure storage mapper - Matched %s -> %s", diskType, ocpStorageClass)
		} else {
			ocpStorageClass = defaultSC
			klog.V(2).Infof("Azure storage mapper - No match for %s, using Azure CSI default SC '%s'", diskType, defaultSC)
		}

		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source: sourceStorage,
			Destination: forkliftv1beta1.DestinationStorage{
				StorageClass: ocpStorageClass,
				VolumeMode:   volumeMode,
				AccessMode:   accessMode,
			},
		})
		klog.V(4).Infof("Azure storage mapper - Mapped %s -> %s (mode: %s, access: %s)",
			diskType, ocpStorageClass, volumeMode, accessMode)
	}

	// Validate all pairs use Azure CSI-backed SCs
	for _, pair := range storagePairs {
		if err := validateAzureCSIProvisioner(pair.Destination.StorageClass, provisioners); err != nil {
			return nil, err
		}
	}

	klog.V(4).Infof("Azure storage mapper - Created %d storage pairs", len(storagePairs))

	storagePairs = mapper.ApplyOffloadToPairs(storagePairs, opts)
	return storagePairs, nil
}

// validateAzureCSIProvisioner checks that a StorageClass has the Azure CSI provisioner.
func validateAzureCSIProvisioner(scName string, provisioners map[string]string) error {
	if len(provisioners) == 0 {
		klog.V(2).Infof("WARNING: Azure storage mapper - No provisioner info available, cannot validate SC '%s'", scName)
		return nil
	}
	prov, exists := provisioners[scName]
	if !exists {
		return fmt.Errorf(
			"StorageClass '%s' not found in target cluster inventory; "+
				"Azure migrations require a StorageClass with provisioner '%s'",
			scName, AzureCSIDriver)
	}
	if prov != AzureCSIDriver {
		return fmt.Errorf(
			"StorageClass '%s' has provisioner '%s', but Azure migrations require provisioner '%s'; "+
				"the migration plan will fail at runtime without the correct CSI driver",
			scName, prov, AzureCSIDriver)
	}
	return nil
}
