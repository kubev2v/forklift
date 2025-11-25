package ebs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"k8s.io/klog/v2"
)

// SnapshotManager handles EBS snapshot operations.
type SnapshotManager struct {
	ec2Client *ec2.Client
	region    string
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(ec2Client *ec2.Client, region string) *SnapshotManager {
	return &SnapshotManager{
		ec2Client: ec2Client,
		region:    region,
	}
}

// SnapshotInfo contains snapshot metadata.
type SnapshotInfo struct {
	SnapshotID       string
	VolumeID         string
	AvailabilityZone string // Original volume's AZ (metadata only - snapshots are region-wide)
	Region           string
	State            string
	Progress         string
	SizeGiB          int32
}

// VerifySnapshot verifies that a snapshot exists and is in a completed state in the region.
// Returns snapshot metadata including size and state.
//
// IMPORTANT: The AZ in snapshot metadata is where the ORIGINAL volume was.
// Snapshots are region-wide - you can create volumes from them in ANY AZ within the region.
func (m *SnapshotManager) VerifySnapshot(ctx context.Context, snapshotID string) (*SnapshotInfo, error) {
	klog.Infof("Verifying snapshot: %s in region: %s", snapshotID, m.region)

	output, err := m.ec2Client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe snapshot: %w", err)
	}

	if len(output.Snapshots) == 0 {
		return nil, fmt.Errorf("snapshot not found: %s in region: %s", snapshotID, m.region)
	}

	snapshot := output.Snapshots[0]
	state := snapshot.State

	if state != types.SnapshotStateCompleted {
		return nil, fmt.Errorf("snapshot not in completed state: %s (current state: %s)", snapshotID, state)
	}

	info := &SnapshotInfo{
		SnapshotID:       aws.ToString(snapshot.SnapshotId),
		VolumeID:         aws.ToString(snapshot.VolumeId),
		AvailabilityZone: aws.ToString(snapshot.AvailabilityZone), // Original volume's AZ (metadata only)
		Region:           m.region,
		State:            string(state),
		Progress:         aws.ToString(snapshot.Progress),
		SizeGiB:          aws.ToInt32(snapshot.VolumeSize),
	}

	klog.Infof("Snapshot verified: %s (state: %s, size: %dGi, original volume AZ: %s)",
		snapshotID, state, info.SizeGiB, info.AvailabilityZone)
	klog.Infof("NOTE: Snapshots are region-wide - can create volume in any AZ within %s", m.region)

	return info, nil
}
