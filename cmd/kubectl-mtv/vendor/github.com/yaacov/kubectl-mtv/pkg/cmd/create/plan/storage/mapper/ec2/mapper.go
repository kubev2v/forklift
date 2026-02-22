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

// findMatchingEBSStorageClass finds the best matching EBS storage class for an EC2 volume type
// Strategy:
// 1. Filter to only EBS storage classes (kubernetes.io/aws-ebs or ebs.csi.aws.com)
// 2. Try to find exact name match (gp3 → gp3 or gp3-csi)
// 3. If not found, use the default EBS storage class
// 4. If no default, use the first EBS storage class found
func findMatchingEBSStorageClass(ec2Type string, targetStorages []forkliftv1beta1.DestinationStorage) string {
	if len(targetStorages) == 0 {
		return ""
	}

	// Separate into EBS and non-EBS storage classes
	var ebsStorageClasses []forkliftv1beta1.DestinationStorage
	var defaultEBSClass string

	for _, storage := range targetStorages {
		// Note: We don't have access to the provisioner here in the current interface
		// We'll use a simple name-based heuristic for EBS detection.
		// This may have false positives for non-EBS classes with similar naming.
		scName := strings.ToLower(storage.StorageClass)

		// Check if this looks like an EBS storage class
		// Look for typical EBS type patterns (gp2, gp3, io1, io2, st1, sc1) or "ebs" keyword
		isEBS := strings.Contains(scName, "ebs") ||
			strings.Contains(scName, "gp2") || strings.Contains(scName, "gp3") ||
			strings.Contains(scName, "io1") || strings.Contains(scName, "io2") ||
			strings.Contains(scName, "st1") || strings.Contains(scName, "sc1")
		if isEBS {
			ebsStorageClasses = append(ebsStorageClasses, storage)

			// Check if this is marked as default (we can't check annotations here, but name might have "default")
			if strings.Contains(scName, "default") {
				defaultEBSClass = storage.StorageClass
			}
		}
	}

	if len(ebsStorageClasses) == 0 {
		klog.V(4).Infof("DEBUG: EC2 storage mapper - No EBS storage classes found, using first available")
		// Find first non-empty storage class name
		for _, storage := range targetStorages {
			if storage.StorageClass != "" {
				return storage.StorageClass
			}
		}
		// If all are empty, return the first one anyway
		return targetStorages[0].StorageClass
	}

	// Try to find exact or close name match
	ec2TypeLower := strings.ToLower(ec2Type)
	for _, storage := range ebsStorageClasses {
		scName := strings.ToLower(storage.StorageClass)

		// Exact match: gp3 → gp3, io2 → io2
		if scName == ec2TypeLower {
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Found exact match: %s → %s", ec2Type, storage.StorageClass)
			return storage.StorageClass
		}

		// Close match with suffix: gp3 → gp3-csi, io2 → io2-csi
		if strings.HasPrefix(scName, ec2TypeLower+"-") {
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Found close match: %s → %s", ec2Type, storage.StorageClass)
			return storage.StorageClass
		}
	}

	// No exact match found, use default EBS class if available
	if defaultEBSClass != "" {
		klog.V(4).Infof("DEBUG: EC2 storage mapper - Using default EBS class: %s → %s", ec2Type, defaultEBSClass)
		return defaultEBSClass
	}

	// No default found, use the first EBS storage class
	klog.V(4).Infof("DEBUG: EC2 storage mapper - Using first EBS class: %s → %s", ec2Type, ebsStorageClasses[0].StorageClass)
	return ebsStorageClasses[0].StorageClass
}

// CreateStoragePairs creates storage mapping pairs for EC2 → OpenShift migrations
// Strategy:
// 1. For each EC2 volume type, find matching EBS storage class by name
// 2. If user specified default, use that for all types
// 3. Otherwise try to match EC2 type to OCP storage class name (gp3→gp3, gp2→gp2)
// 4. If no match, use default EBS storage class
// 5. Set appropriate volume mode (Block/Filesystem) and access mode based on volume type
func (m *EC2StorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	// Validate target provider type - EC2 storage mapping expects OpenShift as target
	if opts.TargetProviderType != "" && opts.TargetProviderType != "openshift" {
		klog.V(2).Infof("WARNING: EC2 storage mapper - Target provider type is '%s', not 'openshift'. EC2→%s migrations may not work as expected.",
			opts.TargetProviderType, opts.TargetProviderType)
	}

	klog.V(4).Infof("DEBUG: EC2 storage mapper - Creating storage pairs for %d source EBS types", len(sourceStorages))

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("DEBUG: No source storages to map")
		return storagePairs, nil
	}

	// If user specified a default storage class, use it for all types
	useDefaultForAll := opts.DefaultTargetStorageClass != ""

	for _, sourceStorage := range sourceStorages {
		ec2VolumeType := sourceStorage.Name

		// Get volume mode and access mode for this EC2 type
		config := getVolumeTypeConfig(ec2VolumeType)

		// Determine target storage class
		var ocpStorageClass string
		if useDefaultForAll {
			// User specified default - use it for all types
			ocpStorageClass = opts.DefaultTargetStorageClass
			klog.V(4).Infof("DEBUG: EC2 storage mapper - Using user-defined storage class '%s' for %s",
				ocpStorageClass, ec2VolumeType)
		} else {
			// Try to find matching EBS storage class
			ocpStorageClass = findMatchingEBSStorageClass(ec2VolumeType, targetStorages)
			if ocpStorageClass == "" {
				klog.V(2).Infof("WARNING: EC2 storage mapper - No target storage class found for %s, skipping", ec2VolumeType)
				continue
			}
		}

		// Create storage pair
		pair := forkliftv1beta1.StoragePair{
			Source: sourceStorage,
			Destination: forkliftv1beta1.DestinationStorage{
				StorageClass: ocpStorageClass,
				VolumeMode:   config.VolumeMode,
				AccessMode:   config.AccessMode,
			},
		}

		storagePairs = append(storagePairs, pair)
		klog.V(4).Infof("DEBUG: EC2 storage mapper - Mapped %s → %s (mode: %s, access: %s)",
			ec2VolumeType, ocpStorageClass, config.VolumeMode, config.AccessMode)
	}

	klog.V(4).Infof("DEBUG: EC2 storage mapper - Created %d storage pairs", len(storagePairs))
	return storagePairs, nil
}
