package collector

import (
	"context"

	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

var azureDiskTypes = []struct {
	SKU         string
	Description string
	MaxIOPS     int32
	MaxMBps     int32
}{
	{"Premium_LRS", "Premium SSD (locally redundant)", 20000, 900},
	{"Standard_LRS", "Standard HDD (locally redundant)", 2000, 500},
	{"StandardSSD_LRS", "Standard SSD (locally redundant)", 6000, 750},
	{"Premium_ZRS", "Premium SSD (zone redundant)", 20000, 900},
	{"StandardSSD_ZRS", "Standard SSD (zone redundant)", 6000, 750},
	{"UltraSSD_LRS", "Ultra Disk (locally redundant)", 160000, 4000},
	{"PremiumV2_LRS", "Premium SSD v2 (locally redundant)", 80000, 1200},
}

func (r *Collector) collectDiskTypes(ctx context.Context) error {
	r.log.V(1).Info("Collecting disk types", "count", len(azureDiskTypes))

	var created, updated, unchanged int
	for _, diskType := range azureDiskTypes {
		m := &model.Storage{}

		m.UID = diskType.SKU
		m.Name = diskType.SKU
		m.Kind = "azure-disk-type"
		m.Provider = string(r.provider.UID)
		m.SKU = diskType.SKU

		m.Object = model.StorageData{
			SKU:         diskType.SKU,
			Description: diskType.Description,
			MaxIOPS:     diskType.MaxIOPS,
			MaxMBps:     diskType.MaxMBps,
		}

		existing := &model.Storage{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			if !existing.HasChanged(m) {
				unchanged++
				continue
			}
			m.Revision = existing.Revision + 1
			if err := r.db.Update(m); err != nil {
				r.log.Error(err, "Failed to update disk type", "sku", diskType.SKU)
				continue
			}
			updated++
		} else {
			m.Revision = 1
			if err := r.db.Insert(m); err != nil {
				r.log.Error(err, "Failed to insert disk type", "sku", diskType.SKU)
				continue
			}
			created++
		}
	}

	r.log.V(1).Info("Disk types processed", "created", created, "updated", updated, "unchanged", unchanged)
	return nil
}
