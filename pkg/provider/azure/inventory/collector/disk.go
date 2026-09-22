package collector

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

func (r *Collector) collectDisks(ctx context.Context) error {
	disks, err := r.client.ListDisks(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected disks", "count", len(disks))

	var created, updated, unchanged int
	for _, azureDisk := range disks {
		if azureDisk == nil || azureDisk.ID == nil {
			continue
		}

		m := &model.Disk{}
		m.UID = *azureDisk.ID

		if azureDisk.Name != nil {
			m.Name = *azureDisk.Name
		} else {
			m.Name = m.UID
		}

		m.Kind = "azure-disk"
		m.Provider = string(r.provider.UID)
		m.DiskType = getDiskType(azureDisk)
		m.State = getDiskState(azureDisk)
		m.SizeGB = getDiskSizeGB(azureDisk)
		m.Object = *azureDisk

		existing := &model.Disk{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			if !existing.HasChanged(m) {
				unchanged++
				continue
			}
			m.Revision = existing.Revision + 1
			if err := r.db.Update(m); err != nil {
				r.log.Error(err, "Failed to update disk", "diskId", m.UID)
				continue
			}
			updated++
		} else {
			m.Revision = 1
			if err := r.db.Insert(m); err != nil {
				r.log.Error(err, "Failed to insert disk", "diskId", m.UID)
				continue
			}
			created++
		}
	}

	r.log.V(1).Info("Disks processed", "created", created, "updated", updated, "unchanged", unchanged)
	return nil
}

func getDiskType(disk *armcompute.Disk) string {
	if disk.SKU != nil && disk.SKU.Name != nil {
		return string(*disk.SKU.Name)
	}
	return ""
}

func getDiskState(disk *armcompute.Disk) string {
	if disk.Properties != nil && disk.Properties.DiskState != nil {
		return strings.ToLower(string(*disk.Properties.DiskState))
	}
	return ""
}

func getDiskSizeGB(disk *armcompute.Disk) int64 {
	if disk.Properties != nil && disk.Properties.DiskSizeGB != nil {
		return int64(*disk.Properties.DiskSizeGB)
	}
	return 0
}
