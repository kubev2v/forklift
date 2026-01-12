package collector

import (
	"context"

	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// EBS volume types (static list)
var ebsVolumeTypes = []struct {
	Type          string
	Description   string
	MaxIOPS       int32
	MaxThroughput int32
}{
	{"gp2", "General Purpose SSD (gp2)", 16000, 250},
	{"gp3", "General Purpose SSD (gp3)", 16000, 1000},
	{"io1", "Provisioned IOPS SSD (io1)", 64000, 1000},
	{"io2", "Provisioned IOPS SSD (io2)", 64000, 1000},
	{"st1", "Throughput Optimized HDD (st1)", 500, 500},
	{"sc1", "Cold HDD (sc1)", 250, 250},
	{"standard", "Magnetic (standard)", 0, 0},
}

// collectStorageTypes collects EBS volume types (static list).
// Context parameter included for consistency with other collection methods,
// though not used since this processes static data (no AWS API calls).
func (r *Collector) collectStorageTypes(ctx context.Context) error {
	r.log.V(1).Info("Collecting storage types", "count", len(ebsVolumeTypes))

	var created, updated, unchanged int
	for _, volType := range ebsVolumeTypes {
		m := &model.Storage{}

		// UID is just the volume type since each provider has its own database
		m.UID = volType.Type
		m.Name = volType.Type // Use type code as name for storage map lookups
		m.Kind = "Storage"
		m.Provider = string(r.provider.UID)
		m.VolumeType = volType.Type

		// Store volume type details as JSON (includes human-readable description)
		details := map[string]interface{}{
			"type":          volType.Type,
			"description":   volType.Description,
			"maxIOPS":       volType.MaxIOPS,
			"maxThroughput": volType.MaxThroughput,
		}

		if err := m.SetObject(details); err != nil {
			r.log.Error(err, "Failed to marshal storage type", "type", volType.Type)
			continue
		}

		// Check if record exists and has changed
		existing := &model.Storage{}
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
				r.log.Error(err, "Failed to update storage type", "type", volType.Type)
				continue
			}
			updated++
		} else {
			// New record - insert
			m.Revision = 1
			if err := r.db.Insert(m); err != nil {
				r.log.Error(err, "Failed to insert storage type", "type", volType.Type)
				continue
			}
			created++
		}
	}

	r.log.V(1).Info("Storage types processed", "created", created, "updated", updated, "unchanged", unchanged)
	return nil
}
