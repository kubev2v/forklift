package ebs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"k8s.io/klog/v2"
)

// VolumeManager handles EBS volume operations.
type VolumeManager struct {
	ec2Client        *ec2.Client
	region           string
	availabilityZone string
}

// NewVolumeManager creates a new EBS volume manager.
func NewVolumeManager(ec2Client *ec2.Client, region, availabilityZone string) *VolumeManager {
	return &VolumeManager{
		ec2Client:        ec2Client,
		region:           region,
		availabilityZone: availabilityZone,
	}
}

// CreateVolumeFromSnapshot creates an EBS volume from a snapshot.
// The volume size is automatically determined from the snapshot - no need to specify it.
func (m *VolumeManager) CreateVolumeFromSnapshot(ctx context.Context, snapshotID string, requestedSizeGiB int32) (string, error) {
	klog.Infof("Creating EBS volume from snapshot: %s in AZ: %s", snapshotID, m.availabilityZone)

	// Get snapshot metadata to validate size
	descOutput, err := m.ec2Client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe snapshot: %w", err)
	}

	if len(descOutput.Snapshots) == 0 {
		return "", fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	snapshot := descOutput.Snapshots[0]
	snapshotSize := aws.ToInt32(snapshot.VolumeSize)

	// Use requested size (with overhead from PVC), but ensure it's not smaller than snapshot
	volumeSize := requestedSizeGiB
	if volumeSize < snapshotSize {
		klog.Warningf("Requested size %dGiB is smaller than snapshot size %dGiB, using snapshot size", volumeSize, snapshotSize)
		volumeSize = snapshotSize
	}

	klog.Infof("Creating volume: requestedSize=%dGiB, snapshotSize=%dGiB, actualSize=%dGiB, AZ=%s",
		requestedSizeGiB, snapshotSize, volumeSize, m.availabilityZone)

	// Create volume from snapshot with explicit size
	// AWS allows creating volumes LARGER than snapshot (adds unallocated space at end)
	createOutput, err := m.ec2Client.CreateVolume(ctx, &ec2.CreateVolumeInput{
		SnapshotId:       aws.String(snapshotID),
		Size:             aws.Int32(volumeSize), // Explicitly set size (can be larger than snapshot)
		AvailabilityZone: aws.String(m.availabilityZone),
		VolumeType:       types.VolumeTypeGp3,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVolume,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("forklift-volume-%s", snapshotID)),
					},
					{
						Key:   aws.String("forklift.konveyor.io/snapshot"),
						Value: aws.String(snapshotID),
					},
					{
						Key:   aws.String("forklift.konveyor.io/created-by"),
						Value: aws.String("ec2-populator"),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create volume: %w", err)
	}

	volumeID := aws.ToString(createOutput.VolumeId)
	klog.Infof("EBS volume created: %s", volumeID)

	return volumeID, nil
}

// WaitForVolumeAvailable waits for volume to become available.
func (m *VolumeManager) WaitForVolumeAvailable(ctx context.Context, volumeID string) error {
	klog.Infof("Waiting for volume: %s", volumeID)

	maxAttempts := 60
	sleepDuration := 5 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		descOutput, err := m.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return fmt.Errorf("failed to describe volume: %w", err)
		}

		if len(descOutput.Volumes) == 0 {
			return fmt.Errorf("volume not found: %s", volumeID)
		}

		volume := descOutput.Volumes[0]
		state := volume.State

		if state == types.VolumeStateAvailable {
			klog.Infof("Volume available: %s", volumeID)
			return nil
		}

		if state == types.VolumeStateError {
			return fmt.Errorf("volume in error state: %s", volumeID)
		}

		if attempt%10 == 0 {
			klog.V(1).Infof("Volume %s state: %s (attempt %d/%d)",
				volumeID, state, attempt+1, maxAttempts)
		}

		time.Sleep(sleepDuration)
	}

	return fmt.Errorf("timeout waiting for volume: %s", volumeID)
}

// GetVolumeInfo retrieves EBS volume information.
func (m *VolumeManager) GetVolumeInfo(ctx context.Context, volumeID string) (*VolumeInfo, error) {
	descOutput, err := m.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe volume: %w", err)
	}

	if len(descOutput.Volumes) == 0 {
		return nil, fmt.Errorf("volume not found: %s", volumeID)
	}

	volume := descOutput.Volumes[0]

	return &VolumeInfo{
		VolumeID:         aws.ToString(volume.VolumeId),
		Size:             aws.ToInt32(volume.Size),
		State:            string(volume.State),
		VolumeType:       string(volume.VolumeType),
		AvailabilityZone: aws.ToString(volume.AvailabilityZone),
		SnapshotID:       aws.ToString(volume.SnapshotId),
	}, nil
}

// VolumeInfo contains EBS volume information.
type VolumeInfo struct {
	VolumeID         string
	Size             int32
	State            string
	VolumeType       string
	AvailabilityZone string
	SnapshotID       string
}
