package ec2

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
)

// EC2StorageMapper implements storage mapping for EC2 providers
type EC2StorageMapper struct{}

// NewEC2StorageMapper creates a new EC2 storage mapper
func NewEC2StorageMapper() mapper.StorageMapper {
	return &EC2StorageMapper{}
}

// EC2VolumeTypeConfig defines configuration for each EC2 EBS volume type
type EC2VolumeTypeConfig struct {
	VolumeMode corev1.PersistentVolumeMode
	AccessMode corev1.PersistentVolumeAccessMode
}

// getVolumeTypeConfig returns the volume mode and access mode for an EC2 volume type
func getVolumeTypeConfig(ec2Type string) EC2VolumeTypeConfig {
	// SSD types use Block mode, HDD types use Filesystem mode
	switch ec2Type {
	case "gp3", "gp2", "io1", "io2":
		// General Purpose and Provisioned IOPS SSDs - use Block mode for best performance
		return EC2VolumeTypeConfig{
			VolumeMode: corev1.PersistentVolumeBlock,
			AccessMode: corev1.ReadWriteOnce,
		}
	case "st1", "sc1", "standard":
		// Throughput Optimized HDD, Cold HDD, Magnetic - use Filesystem mode
		return EC2VolumeTypeConfig{
			VolumeMode: corev1.PersistentVolumeFilesystem,
			AccessMode: corev1.ReadWriteOnce,
		}
	default:
		// Default to Block mode for unknown types
		return EC2VolumeTypeConfig{
			VolumeMode: corev1.PersistentVolumeBlock,
			AccessMode: corev1.ReadWriteOnce,
		}
	}
}

// findMatchingEBSStorageClass tries to find a target SC that matches the given
// EC2 EBS volume type by name. Returns "" when no match is found so the caller
// can fall back to the default SC.
//
// Match strategy:
//  1. Exact name match (gp3 → gp3)
//  2. Prefix match with suffix (gp3 → gp3-csi)
//  3. SC name contains the EBS type as a component (type-gp3, ebs-gp3, etc.)
func findMatchingEBSStorageClass(ec2Type string, targetStorages []forkliftv1beta1.DestinationStorage) string {
	if len(targetStorages) == 0 || ec2Type == "" {
		return ""
	}

	ec2TypeLower := strings.ToLower(ec2Type)

	for _, storage := range targetStorages {
		scName := strings.ToLower(storage.StorageClass)
		if scName == "" {
			continue
		}

		if scName == ec2TypeLower {
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Found exact match: %s → %s", ec2Type, storage.StorageClass)
			return storage.StorageClass
		}

		if strings.HasPrefix(scName, ec2TypeLower+"-") {
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Found prefix match: %s → %s", ec2Type, storage.StorageClass)
			return storage.StorageClass
		}

		if hasDashToken(scName, ec2TypeLower) {
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Found component match: %s → %s", ec2Type, storage.StorageClass)
			return storage.StorageClass
		}
	}

	klog.V(4).Infof("DEBUG: EC2 storage mapper - No name match for %s", ec2Type)
	return ""
}

// hasDashToken checks whether token appears as an exact dash-delimited
// component in s (e.g. hasDashToken("ebs-gp3-csi", "gp3") == true, but
// hasDashToken("provision-io12", "io1") == false).
func hasDashToken(s, token string) bool {
	for _, part := range strings.Split(s, "-") {
		if part == token {
			return true
		}
	}
	return false
}

// CreateStoragePairs creates storage mapping pairs for EC2 → OpenShift migrations.
//
// Three-step flow:
//
//	(a) If the user specified --default-target-storage-class, map every source to that SC.
//	(b) Otherwise, try EBS name-matching against ALL target SCs (gp3→gp3-csi, etc.).
//	(c) After matching, any source still unmapped gets the default SC (targetStorages[0]).
func (m *EC2StorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	if opts.TargetProviderType != "" && opts.TargetProviderType != "openshift" {
		klog.V(2).Infof("WARNING: EC2 storage mapper - Target provider type is '%s', not 'openshift'. EC2→%s migrations may not work as expected.",
			opts.TargetProviderType, opts.TargetProviderType)
	}

	klog.V(4).Infof("DEBUG: EC2 storage mapper - Creating storage pairs for %d source EBS types", len(sourceStorages))

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("DEBUG: No source storages to map")
		return storagePairs, nil
	}

	// (a) User specified a default SC — map every source to it.
	if opts.DefaultTargetStorageClass != "" {
		klog.V(4).Infof("DEBUG: EC2 storage mapper - Using user-defined storage class '%s' for all types", opts.DefaultTargetStorageClass)
		for _, sourceStorage := range sourceStorages {
			config := getVolumeTypeConfig(sourceStorage.Name)
			storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
				Source: sourceStorage,
				Destination: forkliftv1beta1.DestinationStorage{
					StorageClass: opts.DefaultTargetStorageClass,
					VolumeMode:   config.VolumeMode,
					AccessMode:   config.AccessMode,
				},
			})
		}
		klog.V(4).Infof("DEBUG: EC2 storage mapper - Created %d storage pairs (user default)", len(storagePairs))
		return storagePairs, nil
	}

	// Resolve the default SC for gap-filling (best SC selected by the target fetcher).
	defaultSC := ""
	if len(targetStorages) > 0 {
		defaultSC = targetStorages[0].StorageClass
	}

	// (b) Auto-match each EBS type against the full target SC list.
	// (c) If no match, fall back to the default SC.
	for _, sourceStorage := range sourceStorages {
		ec2VolumeType := sourceStorage.Name
		config := getVolumeTypeConfig(ec2VolumeType)

		ocpStorageClass := findMatchingEBSStorageClass(ec2VolumeType, targetStorages)
		if ocpStorageClass != "" {
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Matched %s → %s", ec2VolumeType, ocpStorageClass)
		} else if defaultSC != "" {
			ocpStorageClass = defaultSC
			klog.V(2).Infof("WARNING: EC2 storage mapper - No EBS match for %s, using default SC '%s'", ec2VolumeType, defaultSC)
		} else {
			klog.V(2).Infof("WARNING: EC2 storage mapper - No target storage class available for %s, skipping", ec2VolumeType)
			continue
		}

		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source: sourceStorage,
			Destination: forkliftv1beta1.DestinationStorage{
				StorageClass: ocpStorageClass,
				VolumeMode:   config.VolumeMode,
				AccessMode:   config.AccessMode,
			},
		})
		klog.V(4).Infof("DEBUG: EC2 storage mapper - Mapped %s → %s (mode: %s, access: %s)",
			ec2VolumeType, ocpStorageClass, config.VolumeMode, config.AccessMode)
	}

	klog.V(4).Infof("DEBUG: EC2 storage mapper - Created %d storage pairs", len(storagePairs))
	return storagePairs, nil
}
