package collector

import (
	"context"
	"errors"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// collectSnapshots collects EBS snapshots
func (r *Collector) collectSnapshots(ctx context.Context) error {
	snapshots, err := r.client.DescribeSnapshots(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected snapshots", "count", len(snapshots))

	providerUID := string(r.provider.UID)
	var created, updated, unchanged int
	for _, awsSnapshot := range snapshots {
		m := snapshotFromAWS(awsSnapshot, providerUID)
		if m == nil {
			continue
		}

		action, err := r.upsertSnapshot(m)
		if err != nil {
			r.log.Error(err, "Failed to persist snapshot", "snapshotId", m.ID)
			continue
		}
		switch action {
		case actionCreated:
			created++
		case actionUpdated:
			updated++
		case actionUnchanged:
			unchanged++
		}
	}

	r.log.V(1).Info("Snapshots processed", "created", created, "updated", updated, "unchanged", unchanged)
	return nil
}

// snapshotFromAWS maps an AWS Snapshot to the inventory model.
// Returns nil when the snapshot has no ID and should be skipped.
func snapshotFromAWS(s ec2types.Snapshot, providerUID string) *model.Snapshot {
	if s.SnapshotId == nil {
		return nil
	}

	m := &model.Snapshot{}
	m.ID = *s.SnapshotId

	m.Name = getNameFromTags(s.Tags)
	if m.Name == "" {
		m.Name = m.ID
	}

	m.Kind = "Snapshot"
	m.Provider = providerUID

	m.State = string(s.State)
	if s.VolumeId != nil {
		m.VolumeID = *s.VolumeId
	}

	m.Object = s
	return m
}

// upsertAction describes what upsertSnapshot did.
type upsertAction int

const (
	actionCreated upsertAction = iota
	actionUpdated
	actionUnchanged
)

// upsertSnapshot inserts or updates a snapshot in the database.
// It distinguishes not-found (new record) from other DB errors.
func (r *Collector) upsertSnapshot(m *model.Snapshot) (upsertAction, error) {
	existing := &model.Snapshot{}
	existing.ID = m.ID

	err := r.db.Get(existing)
	switch {
	case err == nil:
		if !existing.HasChanged(m) {
			return actionUnchanged, nil
		}
		m.Revision = existing.Revision + 1
		if err := r.db.Update(m); err != nil {
			return 0, err
		}
		return actionUpdated, nil
	case errors.Is(err, libmodel.NotFound):
		m.Revision = 1
		if err := r.db.Insert(m); err != nil {
			return 0, err
		}
		return actionCreated, nil
	default:
		return 0, err
	}
}
