package ec2

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
)

const EBSCSIDriver = "ebs.csi.aws.com"

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
	switch ec2Type {
	case "gp3", "gp2", "io1", "io2":
		return EC2VolumeTypeConfig{
			VolumeMode: corev1.PersistentVolumeBlock,
			AccessMode: corev1.ReadWriteOnce,
		}
	case "st1", "sc1", "standard":
		return EC2VolumeTypeConfig{
			VolumeMode: corev1.PersistentVolumeFilesystem,
			AccessMode: corev1.ReadWriteOnce,
		}
	default:
		return EC2VolumeTypeConfig{
			VolumeMode: corev1.PersistentVolumeBlock,
			AccessMode: corev1.ReadWriteOnce,
		}
	}
}

// findEBSCSIStorageClass finds the first target SC backed by ebs.csi.aws.com.
func findEBSCSIStorageClass(targetStorages []forkliftv1beta1.DestinationStorage, provisioners map[string]string) string {
	if len(provisioners) == 0 {
		return ""
	}
	for _, storage := range targetStorages {
		if provisioners[storage.StorageClass] == EBSCSIDriver {
			return storage.StorageClass
		}
	}
	return ""
}

// findMatchingEBSStorageClass tries to find a target SC that matches the given
// EC2 EBS volume type by name, preferring EBS CSI-backed SCs.
//
// Match strategy:
//  1. Exact name match with EBS CSI provisioner
//  2. Prefix/component match with EBS CSI provisioner
//  3. Any exact name match
//  4. Any prefix/component match
func findMatchingEBSStorageClass(ec2Type string, targetStorages []forkliftv1beta1.DestinationStorage, provisioners map[string]string) string {
	if len(targetStorages) == 0 || ec2Type == "" {
		return ""
	}

	ec2TypeLower := strings.ToLower(ec2Type)

	// First pass: name matches that are also EBS CSI-backed
	for _, storage := range targetStorages {
		scName := strings.ToLower(storage.StorageClass)
		if scName == "" {
			continue
		}
		if !ebsNameMatches(scName, ec2TypeLower) {
			continue
		}
		if provisioners[storage.StorageClass] == EBSCSIDriver {
			klog.V(4).Infof("EC2 storage mapper - Found CSI name match: %s -> %s", ec2Type, storage.StorageClass)
			return storage.StorageClass
		}
	}

	// Second pass: any name match, but only if it's also EBS CSI-backed
	for _, storage := range targetStorages {
		scName := strings.ToLower(storage.StorageClass)
		if scName == "" {
			continue
		}
		if ebsNameMatches(scName, ec2TypeLower) {
			klog.V(4).Infof("EC2 storage mapper - Found name match (non-CSI): %s -> %s, skipping", ec2Type, storage.StorageClass)
		}
	}

	klog.V(4).Infof("EC2 storage mapper - No name match for %s", ec2Type)
	return ""
}

func ebsNameMatches(scName, ec2TypeLower string) bool {
	if scName == ec2TypeLower {
		return true
	}
	if strings.HasPrefix(scName, ec2TypeLower+"-") {
		return true
	}
	if hasDashToken(scName, ec2TypeLower) {
		return true
	}
	return false
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

// CreateStoragePairs creates storage mapping pairs for EC2 -> OpenShift migrations.
func (m *EC2StorageMapper) CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, opts mapper.StorageMappingOptions) ([]forkliftv1beta1.StoragePair, error) {
	var storagePairs []forkliftv1beta1.StoragePair

	if opts.TargetProviderType != "" && opts.TargetProviderType != "openshift" {
		klog.V(2).Infof("WARNING: EC2 storage mapper - Target provider type is '%s', not 'openshift'. EC2->%s migrations may not work as expected.",
			opts.TargetProviderType, opts.TargetProviderType)
	}

	klog.V(4).Infof("EC2 storage mapper - Creating storage pairs for %d source EBS types", len(sourceStorages))

	if len(sourceStorages) == 0 {
		klog.V(4).Infof("No source storages to map")
		return storagePairs, nil
	}

	provisioners := opts.TargetStorageProvisioners

	// User specified a default SC -- validate and map every source to it
	if opts.DefaultTargetStorageClass != "" {
		klog.V(4).Infof("EC2 storage mapper - Using user-defined storage class '%s' for all types", opts.DefaultTargetStorageClass)

		if err := validateEBSCSIProvisioner(opts.DefaultTargetStorageClass, provisioners); err != nil {
			return nil, err
		}

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
		klog.V(4).Infof("EC2 storage mapper - Created %d storage pairs (user default)", len(storagePairs))

		storagePairs = mapper.ApplyOffloadToPairs(storagePairs, opts)
		return storagePairs, nil
	}

	// Auto-mapping: find the best default SC (must be EBS CSI-backed)
	defaultSC := findEBSCSIStorageClass(targetStorages, provisioners)
	if defaultSC == "" {
		return nil, fmt.Errorf(
			"no StorageClass with provisioner '%s' found on the target cluster; "+
				"EC2 migrations require an EBS CSI-backed StorageClass. "+
				"Use --default-target-storage-class to specify one explicitly, or install the AWS EBS CSI driver",
			EBSCSIDriver)
	}
	klog.V(4).Infof("EC2 storage mapper - Selected EBS CSI default SC: %s", defaultSC)

	for _, sourceStorage := range sourceStorages {
		ec2VolumeType := sourceStorage.Name
		config := getVolumeTypeConfig(ec2VolumeType)

		ocpStorageClass := findMatchingEBSStorageClass(ec2VolumeType, targetStorages, provisioners)
		if ocpStorageClass != "" {
			klog.V(4).Infof("EC2 storage mapper - Matched %s -> %s", ec2VolumeType, ocpStorageClass)
		} else {
			ocpStorageClass = defaultSC
			klog.V(2).Infof("EC2 storage mapper - No match for %s, using EBS CSI default SC '%s'", ec2VolumeType, defaultSC)
		}

		storagePairs = append(storagePairs, forkliftv1beta1.StoragePair{
			Source: sourceStorage,
			Destination: forkliftv1beta1.DestinationStorage{
				StorageClass: ocpStorageClass,
				VolumeMode:   config.VolumeMode,
				AccessMode:   config.AccessMode,
			},
		})
		klog.V(4).Infof("EC2 storage mapper - Mapped %s -> %s (mode: %s, access: %s)",
			ec2VolumeType, ocpStorageClass, config.VolumeMode, config.AccessMode)
	}

	// Validate all pairs use EBS CSI-backed SCs
	for _, pair := range storagePairs {
		if err := validateEBSCSIProvisioner(pair.Destination.StorageClass, provisioners); err != nil {
			return nil, err
		}
	}

	klog.V(4).Infof("EC2 storage mapper - Created %d storage pairs", len(storagePairs))

	storagePairs = mapper.ApplyOffloadToPairs(storagePairs, opts)
	return storagePairs, nil
}

// validateEBSCSIProvisioner checks that a StorageClass has the EBS CSI provisioner.
func validateEBSCSIProvisioner(scName string, provisioners map[string]string) error {
	if len(provisioners) == 0 {
		klog.V(2).Infof("WARNING: EC2 storage mapper - No provisioner info available, cannot validate SC '%s'", scName)
		return nil
	}
	prov, exists := provisioners[scName]
	if !exists {
		return fmt.Errorf(
			"StorageClass '%s' not found in target cluster inventory; "+
				"EC2 migrations require a StorageClass with provisioner '%s'",
			scName, EBSCSIDriver)
	}
	if prov != EBSCSIDriver {
		return fmt.Errorf(
			"StorageClass '%s' has provisioner '%s', but EC2 migrations require provisioner '%s'; "+
				"the migration plan will fail at runtime without the correct CSI driver",
			scName, prov, EBSCSIDriver)
	}
	return nil
}
