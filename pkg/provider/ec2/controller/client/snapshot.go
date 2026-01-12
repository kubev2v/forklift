package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// CreateSnapshot creates EBS snapshots for all volumes attached to an EC2 instance.
// Returns comma-separated snapshot IDs for use in subsequent operations.
// Tags each snapshot with VM ID (for lookups), VM name (for display), and source volume ID.
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (string, string, error) {
	client, err := r.getSourceClient()
	if err != nil {
		return "", "", err
	}

	log.Info("Creating EBS volume snapshots", "vm", vmRef.Name, "id", vmRef.ID)

	ctx := context.Background()

	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{vmRef.ID},
	}

	result, err := client.DescribeInstances(ctx, describeInput)
	if err != nil {
		log.Error(err, "Failed to describe instance", "vm", vmRef.Name)
		return "", "", liberr.Wrap(err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", "", fmt.Errorf("instance not found: %s", vmRef.ID)
	}

	instance := result.Reservations[0].Instances[0]

	snapshotIDs := []string{}
	for _, mapping := range instance.BlockDeviceMappings {
		if mapping.Ebs == nil || mapping.Ebs.VolumeId == nil {
			continue
		}

		volumeID := *mapping.Ebs.VolumeId
		log.Info("Creating snapshot for volume", "vm", vmRef.Name, "volume", volumeID)

		snapshotInput := &ec2.CreateSnapshotInput{
			VolumeId:    aws.String(volumeID),
			Description: aws.String(fmt.Sprintf("Forklift migration snapshot for VM %s (%s)", vmRef.Name, vmRef.ID)),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeSnapshot,
					Tags: []ec2types.Tag{
						{Key: aws.String("forklift.konveyor.io/vmID"), Value: aws.String(vmRef.ID)},
						{Key: aws.String("forklift.konveyor.io/vm-name"), Value: aws.String(vmRef.Name)},
						{Key: aws.String("forklift.konveyor.io/volume"), Value: aws.String(volumeID)},
					},
				},
			},
		}

		snapshot, err := client.CreateSnapshot(ctx, snapshotInput)
		if err != nil {
			log.Error(err, "Failed to create snapshot", "volume", volumeID)
			return "", "", liberr.Wrap(err)
		}

		snapshotIDs = append(snapshotIDs, *snapshot.SnapshotId)
		log.Info("Snapshot created", "vm", vmRef.Name, "volume", volumeID, "snapshot", *snapshot.SnapshotId)
	}

	snapshotIDString := strings.Join(snapshotIDs, ",")

	log.Info("All snapshots created", "vm", vmRef.Name, "snapshots", snapshotIDString)
	return snapshotIDString, "", nil
}

// RemoveSnapshot deletes EBS snapshots specified in comma-separated format.
// Returns empty string on success. Skips deletion if snapshot string is empty.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (string, error) {
	client, err := r.getSourceClient()
	if err != nil {
		return "", err
	}

	if snapshot == "" {
		return "", nil
	}

	log.Info("Removing EBS snapshots", "vm", vmRef.Name, "snapshots", snapshot)

	ctx := context.Background()
	snapshotIDs := splitSnapshotIDs(snapshot)

	for _, snapshotID := range snapshotIDs {
		input := &ec2.DeleteSnapshotInput{
			SnapshotId: aws.String(snapshotID),
		}

		_, err := client.DeleteSnapshot(ctx, input)
		if err != nil {
			log.Error(err, "Failed to delete snapshot", "snapshot", snapshotID)
			return "", liberr.Wrap(err)
		}

		log.Info("Snapshot deleted", "snapshot", snapshotID)
	}

	log.Info("All snapshots removed", "vm", vmRef.Name)
	return "", nil
}

// CheckSnapshotReady polls EBS snapshots to verify they've reached completed state.
// Returns true when all snapshots are complete and ready for volume creation.
// Logs progress percentage for snapshots still in progress.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, string, error) {
	client, err := r.getSourceClient()
	if err != nil {
		return false, "", err
	}

	snapshotIDs := splitSnapshotIDs(precopy.Snapshot)
	if len(snapshotIDs) == 0 {
		return false, "", fmt.Errorf("no snapshot IDs found")
	}

	ctx := context.Background()
	input := &ec2.DescribeSnapshotsInput{
		SnapshotIds: snapshotIDs,
	}

	result, err := client.DescribeSnapshots(ctx, input)
	if err != nil {
		log.Error(err, "Failed to describe snapshots", "vm", vmRef.Name)
		return false, "", liberr.Wrap(err)
	}

	allReady := true
	for _, snapshot := range result.Snapshots {
		if snapshot.State != ec2types.SnapshotStateCompleted {
			allReady = false
			log.V(2).Info("Snapshot not ready",
				"snapshot", *snapshot.SnapshotId,
				"state", snapshot.State,
				"progress", aws.ToString(snapshot.Progress))
		}
	}

	if allReady {
		log.Info("All snapshots ready", "vm", vmRef.Name, "count", len(result.Snapshots))
	}

	return allReady, precopy.Snapshot, nil
}

// CheckSnapshotRemove verifies snapshots have been successfully deleted.
// Returns true if snapshots are not found (already deleted) or if none exist.
// Handles InvalidSnapshot.NotFound errors gracefully as successful deletion.
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	client, err := r.getSourceClient()
	if err != nil {
		return false, err
	}

	snapshotIDs := splitSnapshotIDs(precopy.Snapshot)
	if len(snapshotIDs) == 0 {
		return true, nil
	}

	ctx := context.Background()
	input := &ec2.DescribeSnapshotsInput{
		SnapshotIds: snapshotIDs,
	}

	result, err := client.DescribeSnapshots(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidSnapshot.NotFound") {
			r.Log.V(2).Info("Snapshots not found (already deleted)", "vmRef", vmRef.Name)
			return true, nil
		}
		r.Log.Error(err, "Failed to check snapshot status", "vmRef", vmRef.Name)
		return false, liberr.Wrap(err)
	}

	return len(result.Snapshots) == 0, nil
}

// splitSnapshotIDs parses comma-separated snapshot IDs into a slice.
// Returns empty slice for empty input string.
func splitSnapshotIDs(snapshotIDString string) []string {
	if snapshotIDString == "" {
		return []string{}
	}
	return strings.Split(snapshotIDString, ",")
}

// GetSnapshotsForVM queries AWS for all snapshots tagged with the given VM ID.
// Returns a map of volumeID -> snapshotID by reading tags from each snapshot.
func (r *Client) GetSnapshotsForVM(vmRef ref.Ref) (map[string]string, error) {
	client, err := r.getSourceClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	input := &ec2.DescribeSnapshotsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("tag:forklift.konveyor.io/vmID"),
				Values: []string{vmRef.ID},
			},
		},
	}

	result, err := client.DescribeSnapshots(ctx, input)
	if err != nil {
		log.Error(err, "Failed to query snapshots for VM", "vm", vmRef.Name, "id", vmRef.ID)
		return nil, liberr.Wrap(err)
	}

	// Build volumeID -> snapshotID mapping from snapshot tags
	snapshotMap := make(map[string]string)
	for _, snapshot := range result.Snapshots {
		snapshotID := aws.ToString(snapshot.SnapshotId)
		volumeID := ""

		// Find the volume tag
		for _, tag := range snapshot.Tags {
			if aws.ToString(tag.Key) == "forklift.konveyor.io/volume" {
				volumeID = aws.ToString(tag.Value)
				break
			}
		}

		if volumeID != "" && snapshotID != "" {
			snapshotMap[volumeID] = snapshotID
			log.V(2).Info("Found snapshot for volume",
				"vm", vmRef.Name,
				"volumeID", volumeID,
				"snapshotID", snapshotID)
		}
	}

	log.Info("Retrieved snapshots for VM from AWS tags",
		"vm", vmRef.Name,
		"id", vmRef.ID,
		"count", len(snapshotMap))

	return snapshotMap, nil
}

// GetSnapshotIDsForVM returns a comma-separated string of snapshot IDs for the VM.
// Convenience method for use with CheckSnapshotReady which expects this format.
func (r *Client) GetSnapshotIDsForVM(vmRef ref.Ref) (string, error) {
	snapshotMap, err := r.GetSnapshotsForVM(vmRef)
	if err != nil {
		return "", err
	}

	snapshotIDs := make([]string, 0, len(snapshotMap))
	for _, snapshotID := range snapshotMap {
		snapshotIDs = append(snapshotIDs, snapshotID)
	}

	return strings.Join(snapshotIDs, ","), nil
}

// GetCreatedVolumesForVM queries AWS for EBS volumes created during migration for this VM.
// Returns a map of originalVolumeID -> newVolumeID by reading tags from each volume.
func (r *Client) GetCreatedVolumesForVM(vmRef ref.Ref) (map[string]string, error) {
	// Use targetClient as volumes are created in the target account
	client, err := r.getTargetClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	input := &ec2.DescribeVolumesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("tag:forklift.konveyor.io/vmID"),
				Values: []string{vmRef.ID},
			},
			{
				// Only get volumes that have the original-volume tag (created by migration)
				Name:   aws.String("tag-key"),
				Values: []string{"forklift.konveyor.io/original-volume"},
			},
		},
	}

	result, err := client.DescribeVolumes(ctx, input)
	if err != nil {
		log.Error(err, "Failed to query created volumes for VM", "vm", vmRef.Name, "id", vmRef.ID)
		return nil, liberr.Wrap(err)
	}

	// Build originalVolumeID -> newVolumeID mapping from volume tags
	volumeMap := make(map[string]string)
	for _, volume := range result.Volumes {
		newVolumeID := aws.ToString(volume.VolumeId)
		originalVolumeID := ""

		// Find the original-volume tag
		for _, tag := range volume.Tags {
			if aws.ToString(tag.Key) == "forklift.konveyor.io/original-volume" {
				originalVolumeID = aws.ToString(tag.Value)
				break
			}
		}

		if originalVolumeID != "" && newVolumeID != "" {
			volumeMap[originalVolumeID] = newVolumeID
			log.V(2).Info("Found created volume",
				"vm", vmRef.Name,
				"originalVolumeID", originalVolumeID,
				"newVolumeID", newVolumeID)
		}
	}

	log.Info("Retrieved created volumes for VM from AWS tags",
		"vm", vmRef.Name,
		"id", vmRef.ID,
		"count", len(volumeMap))

	return volumeMap, nil
}

// CreateVolumeFromSnapshot creates a new EBS volume from a snapshot in the target AZ.
// Preserves the original volume type and creates the volume in the configured target-az.
// In cross-account mode, uses targetClient to create the volume in the target account.
// Returns the new volume ID.
func (r *Client) CreateVolumeFromSnapshot(vmRef ref.Ref, originalVolumeID, snapshotID string) (string, error) {
	sourceClient, err := r.getSourceClient()
	if err != nil {
		log.Error(err, "Source EC2 client not available", "vm", vmRef.Name)
		return "", err
	}

	targetClient, err := r.getTargetClient()
	if err != nil {
		log.Error(err, "Target EC2 client not available", "vm", vmRef.Name)
		return "", err
	}

	log.Info("Starting volume creation from snapshot",
		"vm", vmRef.Name,
		"snapshotID", snapshotID,
		"originalVolumeID", originalVolumeID,
		"crossAccount", r.crossAccount)

	ctx := context.Background()

	// Verify the snapshot exists using target client (it should see shared snapshot)
	describeSnapInput := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
	}

	snapResult, err := targetClient.DescribeSnapshots(ctx, describeSnapInput)
	if err != nil {
		log.Error(err, "Failed to describe snapshot from target account",
			"vm", vmRef.Name,
			"snapshotID", snapshotID)
		return "", liberr.Wrap(err)
	}

	if len(snapResult.Snapshots) == 0 {
		err := fmt.Errorf("snapshot not found in target account: %s", snapshotID)
		log.Error(err, "Snapshot does not exist or not shared", "vm", vmRef.Name)
		return "", err
	}

	snapshot := snapResult.Snapshots[0]
	log.Info("Snapshot verified in target account",
		"vm", vmRef.Name,
		"snapshotID", snapshotID,
		"state", snapshot.State,
		"progress", *snapshot.Progress)

	// Get the original volume info from SOURCE account to preserve its type
	describeVolInput := &ec2.DescribeVolumesInput{
		VolumeIds: []string{originalVolumeID},
	}

	volResult, err := sourceClient.DescribeVolumes(ctx, describeVolInput)
	if err != nil {
		log.Error(err, "Failed to describe original volume",
			"vm", vmRef.Name,
			"volumeID", originalVolumeID)
		return "", liberr.Wrap(err)
	}

	if len(volResult.Volumes) == 0 {
		err := fmt.Errorf("original volume not found: %s", originalVolumeID)
		log.Error(err, "Original volume does not exist", "vm", vmRef.Name)
		return "", err
	}

	originalVolume := volResult.Volumes[0]
	log.Info("Original volume verified",
		"vm", vmRef.Name,
		"volumeID", originalVolumeID,
		"originalAZ", *originalVolume.AvailabilityZone,
		"volumeType", originalVolume.VolumeType)

	targetAZ, err := r.getTargetClusterAZ()
	if err != nil {
		log.Error(err, "Failed to get target cluster AZ", "vm", vmRef.Name)
		return "", liberr.Wrap(err)
	}

	log.Info("Target cluster AZ determined",
		"vm", vmRef.Name,
		"targetAZ", targetAZ,
		"originalAZ", *originalVolume.AvailabilityZone,
		"azMatch", targetAZ == *originalVolume.AvailabilityZone)

	createVolInput := &ec2.CreateVolumeInput{
		SnapshotId:       aws.String(snapshotID),
		AvailabilityZone: aws.String(targetAZ),
		VolumeType:       originalVolume.VolumeType, // Preserve volume type
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeVolume,
				Tags: []ec2types.Tag{
					{Key: aws.String("forklift.konveyor.io/vmID"), Value: aws.String(vmRef.ID)},
					{Key: aws.String("forklift.konveyor.io/vm-name"), Value: aws.String(vmRef.Name)},
					{Key: aws.String("forklift.konveyor.io/original-volume"), Value: aws.String(originalVolumeID)},
					{Key: aws.String("forklift.konveyor.io/snapshot"), Value: aws.String(snapshotID)},
				},
			},
		},
	}

	// Create volume in TARGET account
	volume, err := targetClient.CreateVolume(ctx, createVolInput)
	if err != nil {
		log.Error(err, "Failed to create volume from snapshot", "snapshotID", snapshotID, "targetAZ", targetAZ)
		return "", liberr.Wrap(err)
	}

	newVolumeID := *volume.VolumeId
	log.Info("Volume created from snapshot in target account",
		"vm", vmRef.Name,
		"snapshotID", snapshotID,
		"originalVolumeID", originalVolumeID,
		"newVolumeID", newVolumeID,
		"targetAZ", targetAZ,
		"crossAccount", r.crossAccount)

	return newVolumeID, nil
}

// getTargetClusterAZ retrieves the target availability zone from provider settings.
// The target-az must be configured in provider settings for volume creation.
func (r *Client) getTargetClusterAZ() (string, error) {
	if r.Source.Provider == nil {
		return "", fmt.Errorf("source provider is nil")
	}

	if r.Source.Provider.Spec.Settings == nil {
		return "", fmt.Errorf("provider spec.settings is not configured, target-az is required")
	}

	targetAZ, ok := r.Source.Provider.Spec.Settings["target-az"]
	if !ok || targetAZ == "" {
		return "", fmt.Errorf("provider spec.settings.target-az is not configured, must specify target availability zone")
	}

	log.Info("Using target AZ from provider settings", "az", targetAZ)
	return targetAZ, nil
}

// CheckVolumesReady polls EBS volumes to verify they've reached available state.
// Returns true when all volumes are ready for attachment or use.
// Uses targetClient as volumes are created in the target account.
func (r *Client) CheckVolumesReady(vmRef ref.Ref, volumeIDs []string) (bool, error) {
	client, err := r.getTargetClient()
	if err != nil {
		return false, err
	}

	if len(volumeIDs) == 0 {
		return false, fmt.Errorf("no volume IDs provided")
	}

	ctx := context.Background()
	input := &ec2.DescribeVolumesInput{
		VolumeIds: volumeIDs,
	}

	result, err := client.DescribeVolumes(ctx, input)
	if err != nil {
		log.Error(err, "Failed to describe volumes", "vm", vmRef.Name)
		return false, liberr.Wrap(err)
	}

	allReady := true
	for _, volume := range result.Volumes {
		if volume.State != ec2types.VolumeStateAvailable {
			allReady = false
			log.V(2).Info("Volume not ready",
				"volumeID", *volume.VolumeId,
				"state", volume.State)
		}
	}

	if allReady {
		log.Info("All volumes ready", "vm", vmRef.Name, "count", len(result.Volumes))
	}

	return allReady, nil
}

// RemoveVolumes deletes EBS volumes by ID.
// Handles InvalidVolume.NotFound errors gracefully (already deleted).
// Returns error on first failure to delete.
// Uses targetClient as volumes are created in the target account.
func (r *Client) RemoveVolumes(vmRef ref.Ref, volumeIDs []string) error {
	client, err := r.getTargetClient()
	if err != nil {
		return err
	}

	if len(volumeIDs) == 0 {
		return nil
	}

	log.Info("Removing EBS volumes", "vm", vmRef.Name, "count", len(volumeIDs))

	ctx := context.Background()

	for _, volumeID := range volumeIDs {
		input := &ec2.DeleteVolumeInput{
			VolumeId: aws.String(volumeID),
		}

		_, err = client.DeleteVolume(ctx, input)
		if err != nil {
			// Check if volume is already deleted
			if strings.Contains(err.Error(), "InvalidVolume.NotFound") {
				log.V(2).Info("Volume already deleted", "volumeID", volumeID)
				continue
			}
			log.Error(err, "Failed to delete volume", "volumeID", volumeID)
			return liberr.Wrap(err)
		}

		log.Info("Volume deleted", "volumeID", volumeID)
	}

	log.Info("All volumes removed", "vm", vmRef.Name)
	return nil
}

// ShareSnapshot shares a snapshot with the target AWS account for cross-account migration.
// This allows the target account to create volumes from the snapshot.
func (r *Client) ShareSnapshot(snapshotID, targetAccountID string) error {
	client, err := r.getSourceClient()
	if err != nil {
		return err
	}

	log.Info("Sharing snapshot with target account",
		"snapshotID", snapshotID,
		"targetAccountID", targetAccountID)

	ctx := context.Background()
	input := &ec2.ModifySnapshotAttributeInput{
		SnapshotId: aws.String(snapshotID),
		Attribute:  ec2types.SnapshotAttributeNameCreateVolumePermission,
		CreateVolumePermission: &ec2types.CreateVolumePermissionModifications{
			Add: []ec2types.CreateVolumePermission{
				{UserId: aws.String(targetAccountID)},
			},
		},
	}

	_, err = client.ModifySnapshotAttribute(ctx, input)
	if err != nil {
		log.Error(err, "Failed to share snapshot",
			"snapshotID", snapshotID,
			"targetAccountID", targetAccountID)
		return liberr.Wrap(err)
	}

	log.Info("Snapshot shared successfully",
		"snapshotID", snapshotID,
		"targetAccountID", targetAccountID)
	return nil
}

// UnshareSnapshot removes snapshot sharing permission from a target account.
// Called during cleanup after successful migration.
func (r *Client) UnshareSnapshot(snapshotID, targetAccountID string) error {
	client, err := r.getSourceClient()
	if err != nil {
		return err
	}

	log.Info("Removing snapshot sharing",
		"snapshotID", snapshotID,
		"targetAccountID", targetAccountID)

	ctx := context.Background()
	input := &ec2.ModifySnapshotAttributeInput{
		SnapshotId: aws.String(snapshotID),
		Attribute:  ec2types.SnapshotAttributeNameCreateVolumePermission,
		CreateVolumePermission: &ec2types.CreateVolumePermissionModifications{
			Remove: []ec2types.CreateVolumePermission{
				{UserId: aws.String(targetAccountID)},
			},
		},
	}

	_, err = client.ModifySnapshotAttribute(ctx, input)
	if err != nil {
		// Log but don't fail - snapshot might already be deleted
		log.V(2).Info("Failed to remove snapshot sharing (may already be unshared)",
			"snapshotID", snapshotID,
			"error", err.Error())
		return nil
	}

	log.Info("Snapshot sharing removed",
		"snapshotID", snapshotID,
		"targetAccountID", targetAccountID)
	return nil
}
