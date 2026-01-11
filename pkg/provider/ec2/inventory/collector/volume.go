package collector

import (
	"context"

	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// collectVolumes collects EBS volumes
func (r *Collector) collectVolumes(ctx context.Context) error {
	volumes, err := r.client.DescribeVolumes(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected volumes", "count", len(volumes))

	var created, updated, unchanged int
	for _, awsVolume := range volumes {
		m := &model.Volume{}

		// Set minimal indexed fields
		if awsVolume.VolumeId != nil {
			m.UID = *awsVolume.VolumeId
		} else {
			continue // Skip volumes without ID
		}

		m.Name = getNameFromTags(awsVolume.Tags)
		if m.Name == "" {
			m.Name = m.UID // Use volume ID as name if no Name tag
		}

		m.Kind = "Volume"
		m.Provider = string(r.provider.UID)

		// Set EBS-specific indexed fields
		m.VolumeType = string(awsVolume.VolumeType)
		m.State = string(awsVolume.State)
		if awsVolume.Size != nil {
			m.Size = int64(*awsVolume.Size) // Size in GiB
		}

		// Store complete AWS volume as JSON
		if err := m.SetObject(awsVolume); err != nil {
			r.log.Error(err, "Failed to marshal volume", "volumeId", m.UID)
			continue
		}

		// Check if record exists and has changed
		existing := &model.Volume{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			// Record exists - check if it changed
			if !existing.HasChanged(m) {
				unchanged++
				continue // No change, skip DB write
			}
			// Changed - update with incremented revision
			m.Revision = existing.Revision + 1
			if err := r.db.Update(m); err != nil {
				r.log.Error(err, "Failed to update volume", "volumeId", m.UID)
				continue
			}
			updated++
		} else {
			// New record - insert
			m.Revision = 1
			if err := r.db.Insert(m); err != nil {
				r.log.Error(err, "Failed to insert volume", "volumeId", m.UID)
				continue
			}
			created++
		}
	}

	r.log.V(1).Info("Volumes processed", "created", created, "updated", updated, "unchanged", unchanged)
	return nil
}
