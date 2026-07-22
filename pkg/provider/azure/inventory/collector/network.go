package collector

import (
	"context"

	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

func (r *Collector) collectNetworks(ctx context.Context) error {
	var totalCreated, totalUpdated, totalUnchanged int

	vnets, err := r.client.ListVirtualNetworks(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected VNets", "count", len(vnets))

	for _, vnet := range vnets {
		if vnet == nil || vnet.ID == nil || vnet.Name == nil {
			continue
		}

		subnets, err := r.client.ListSubnets(ctx, *vnet.Name)
		if err != nil {
			r.log.Error(err, "Failed to list subnets", "vnet", *vnet.Name)
			continue
		}

		for _, subnet := range subnets {
			if subnet == nil || subnet.ID == nil {
				continue
			}

			m := &model.Network{}
			m.UID = *subnet.ID

			if subnet.Name != nil {
				m.Name = *subnet.Name
			} else {
				m.Name = m.UID
			}

			m.Kind = "azure-network"
			m.Provider = string(r.provider.UID)
			m.NetworkType = "subnet"

			if subnet.Properties != nil && subnet.Properties.AddressPrefix != nil {
				m.AddressPrefix = *subnet.Properties.AddressPrefix
			}

			m.Object = *subnet

			existing := &model.Network{}
			existing.UID = m.UID
			if err := r.db.Get(existing); err == nil {
				if !existing.HasChanged(m) {
					totalUnchanged++
					continue
				}
				m.Revision = existing.Revision + 1
				if err := r.db.Update(m); err != nil {
					r.log.Error(err, "Failed to update subnet", "subnetId", m.UID)
					continue
				}
				totalUpdated++
			} else {
				m.Revision = 1
				if err := r.db.Insert(m); err != nil {
					r.log.Error(err, "Failed to insert subnet", "subnetId", m.UID)
					continue
				}
				totalCreated++
			}
		}
	}

	r.log.V(1).Info("Networks processed", "created", totalCreated, "updated", totalUpdated, "unchanged", totalUnchanged)
	return nil
}
