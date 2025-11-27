package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// refresh orchestrates the periodic inventory refresh from the dynamic provider.
// It fetches VMs, networks, storage, and disks, compares them with cached data,
// and updates the SQLite cache with any changes (created/updated/deleted).
func (r *Collector) refresh(ctx context.Context) (err error) {
	if r.config == nil {
		return fmt.Errorf("no config found")
	}

	mark := time.Now()

	// Refresh VMs
	vmsStats, err := r.refreshVMs()
	if err != nil {
		return err
	}

	// Refresh Networks
	networksStats, err := r.refreshNetworks()
	if err != nil {
		return err
	}

	// Refresh Storage
	storageStats, err := r.refreshStorage()
	if err != nil {
		return err
	}

	// Refresh Disks
	disksStats, err := r.refreshDisks()
	if err != nil {
		return err
	}

	totalChanges := vmsStats[0] + vmsStats[1] + vmsStats[2] +
		networksStats[0] + networksStats[1] + networksStats[2] +
		storageStats[0] + storageStats[1] + storageStats[2] +
		disksStats[0] + disksStats[1] + disksStats[2]

	if totalChanges > 0 {
		log.Info("Inventory refresh complete",
			"provider", r.provider.Name,
			"vms_created", vmsStats[0],
			"vms_updated", vmsStats[1],
			"vms_deleted", vmsStats[2],
			"networks_created", networksStats[0],
			"networks_updated", networksStats[1],
			"networks_deleted", networksStats[2],
			"storage_created", storageStats[0],
			"storage_updated", storageStats[1],
			"storage_deleted", storageStats[2],
			"disks_created", disksStats[0],
			"disks_updated", disksStats[1],
			"disks_deleted", disksStats[2],
			"duration", time.Since(mark))
	} else {
		log.V(3).Info("Inventory refresh complete (no changes)",
			"provider", r.provider.Name,
			"duration", time.Since(mark))
	}

	return nil
}

// refreshVMs fetches VMs from the external provider, compares with cached data,
// and updates the cache. Returns [created, updated, deleted] counts.
func (r *Collector) refreshVMs() ([3]int, error) {
	stats := [3]int{0, 0, 0}

	// Fetch updated inventory from external service
	vmsURL := r.config.ServiceURL + "/vms"
	resp, err := http.Get(vmsURL)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to refresh VMs")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return stats, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to read response")
	}

	var vmsData []map[string]interface{}
	err = json.Unmarshal(body, &vmsData)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to parse VMs JSON")
	}

	// Build map of current VMs from provider
	currentVMs := make(map[string]map[string]interface{})
	for _, vmData := range vmsData {
		if id, ok := vmData["id"].(string); ok {
			currentVMs[id] = vmData
		}
	}

	// Get existing VMs from DB
	tx, err := r.db.Begin()
	if err != nil {
		return stats, err
	}
	defer func() {
		_ = tx.End()
	}()

	existingVMs := []dynamic.VM{}
	err = tx.List(&existingVMs, libmodel.ListOptions{})
	if err != nil {
		return stats, err
	}

	existingMap := make(map[string]*dynamic.VM)
	for i := range existingVMs {
		existingMap[existingVMs[i].ID] = &existingVMs[i]
	}

	// Check for new and updated VMs
	for id, vmData := range currentVMs {
		newVM, err := r.parseVM(vmData)
		if err != nil {
			log.V(3).Info("Failed to parse VM, skipping",
				"error", err,
				"id", id)
			continue
		}

		if existing, found := existingMap[id]; found {
			if r.vmChanged(existing, newVM) {
				err = tx.Update(newVM)
				if err != nil {
					log.V(3).Info("Failed to update VM",
						"error", err,
						"id", id)
				} else {
					stats[1]++ // updated
				}
			}
		} else {
			err = tx.Insert(newVM)
			if err != nil {
				log.V(3).Info("Failed to insert VM",
					"error", err,
					"id", id)
			} else {
				stats[0]++ // created
			}
		}

		delete(existingMap, id)
	}

	// Remaining VMs in existingMap were deleted from provider
	for id, vm := range existingMap {
		err = tx.Delete(vm)
		if err != nil {
			log.V(3).Info("Failed to delete VM",
				"error", err,
				"id", id)
		} else {
			stats[2]++ // deleted
		}
	}

	err = tx.Commit()
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// refreshNetworks fetches networks from the external provider, compares with cached data,
// and updates the cache. Returns [created, updated, deleted] counts.
func (r *Collector) refreshNetworks() ([3]int, error) {
	stats := [3]int{0, 0, 0}

	networksURL := r.config.ServiceURL + "/networks"
	resp, err := http.Get(networksURL)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to refresh networks")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return stats, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to read response")
	}

	var networksData []map[string]interface{}
	err = json.Unmarshal(body, &networksData)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to parse networks JSON")
	}

	currentNetworks := make(map[string]map[string]interface{})
	for _, networkData := range networksData {
		if id, ok := networkData["id"].(string); ok {
			currentNetworks[id] = networkData
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return stats, err
	}
	defer func() {
		_ = tx.End()
	}()

	existingNetworks := []dynamic.Network{}
	err = tx.List(&existingNetworks, libmodel.ListOptions{})
	if err != nil {
		return stats, err
	}

	existingMap := make(map[string]*dynamic.Network)
	for i := range existingNetworks {
		existingMap[existingNetworks[i].ID] = &existingNetworks[i]
	}

	for id, networkData := range currentNetworks {
		newNetwork, err := r.parseNetwork(networkData)
		if err != nil {
			log.V(3).Info("Failed to parse network, skipping",
				"error", err,
				"id", id)
			continue
		}

		if existing, found := existingMap[id]; found {
			if r.networkChanged(existing, newNetwork) {
				err = tx.Update(newNetwork)
				if err != nil {
					log.V(3).Info("Failed to update network",
						"error", err,
						"id", id)
				} else {
					stats[1]++ // updated
				}
			}
		} else {
			err = tx.Insert(newNetwork)
			if err != nil {
				log.V(3).Info("Failed to insert network",
					"error", err,
					"id", id)
			} else {
				stats[0]++ // created
			}
		}

		delete(existingMap, id)
	}

	for id, network := range existingMap {
		err = tx.Delete(network)
		if err != nil {
			log.V(3).Info("Failed to delete network",
				"error", err,
				"id", id)
		} else {
			stats[2]++ // deleted
		}
	}

	err = tx.Commit()
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// refreshStorage fetches storage from the external provider, compares with cached data,
// and updates the cache. Returns [created, updated, deleted] counts.
func (r *Collector) refreshStorage() ([3]int, error) {
	stats := [3]int{0, 0, 0}

	storageURL := r.config.ServiceURL + "/storages"
	resp, err := http.Get(storageURL)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to refresh storage")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return stats, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to read response")
	}

	var storageData []map[string]interface{}
	err = json.Unmarshal(body, &storageData)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to parse storage JSON")
	}

	currentStorage := make(map[string]map[string]interface{})
	for _, storData := range storageData {
		if id, ok := storData["id"].(string); ok {
			currentStorage[id] = storData
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return stats, err
	}
	defer func() {
		_ = tx.End()
	}()

	existingStorage := []dynamic.Storage{}
	err = tx.List(&existingStorage, libmodel.ListOptions{})
	if err != nil {
		return stats, err
	}

	existingMap := make(map[string]*dynamic.Storage)
	for i := range existingStorage {
		existingMap[existingStorage[i].ID] = &existingStorage[i]
	}

	for id, storData := range currentStorage {
		newStorage, err := r.parseStorage(storData)
		if err != nil {
			log.V(3).Info("Failed to parse storage, skipping",
				"error", err,
				"id", id)
			continue
		}

		if existing, found := existingMap[id]; found {
			if r.storageChanged(existing, newStorage) {
				err = tx.Update(newStorage)
				if err != nil {
					log.V(3).Info("Failed to update storage",
						"error", err,
						"id", id)
				} else {
					stats[1]++ // updated
				}
			}
		} else {
			err = tx.Insert(newStorage)
			if err != nil {
				log.V(3).Info("Failed to insert storage",
					"error", err,
					"id", id)
			} else {
				stats[0]++ // created
			}
		}

		delete(existingMap, id)
	}

	for id, storage := range existingMap {
		err = tx.Delete(storage)
		if err != nil {
			log.V(3).Info("Failed to delete storage",
				"error", err,
				"id", id)
		} else {
			stats[2]++ // deleted
		}
	}

	err = tx.Commit()
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// refreshDisks fetches disks from the external provider, compares with cached data,
// and updates the cache. Returns [created, updated, deleted] counts.
func (r *Collector) refreshDisks() ([3]int, error) {
	stats := [3]int{0, 0, 0}

	disksURL := r.config.ServiceURL + "/disks"
	resp, err := http.Get(disksURL)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to refresh disks")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return stats, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to read response")
	}

	var disksData []map[string]interface{}
	err = json.Unmarshal(body, &disksData)
	if err != nil {
		return stats, liberr.Wrap(err, "failed to parse disks JSON")
	}

	currentDisks := make(map[string]map[string]interface{})
	for _, diskData := range disksData {
		if id, ok := diskData["id"].(string); ok {
			currentDisks[id] = diskData
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return stats, err
	}
	defer func() {
		_ = tx.End()
	}()

	existingDisks := []dynamic.Disk{}
	err = tx.List(&existingDisks, libmodel.ListOptions{})
	if err != nil {
		return stats, err
	}

	existingMap := make(map[string]*dynamic.Disk)
	for i := range existingDisks {
		existingMap[existingDisks[i].ID] = &existingDisks[i]
	}

	for id, diskData := range currentDisks {
		newDisk, err := r.parseDisk(diskData)
		if err != nil {
			log.V(3).Info("Failed to parse disk, skipping",
				"error", err,
				"id", id)
			continue
		}

		if existing, found := existingMap[id]; found {
			if r.diskChanged(existing, newDisk) {
				err = tx.Update(newDisk)
				if err != nil {
					log.V(3).Info("Failed to update disk",
						"error", err,
						"id", id)
				} else {
					stats[1]++ // updated
				}
			}
		} else {
			err = tx.Insert(newDisk)
			if err != nil {
				log.V(3).Info("Failed to insert disk",
					"error", err,
					"id", id)
			} else {
				stats[0]++ // created
			}
		}
		delete(existingMap, id)
	}

	for id, disk := range existingMap {
		err = tx.Delete(disk)
		if err != nil {
			log.V(3).Info("Failed to delete disk",
				"error", err,
				"id", id)
		} else {
			stats[2]++ // deleted
		}
	}

	err = tx.Commit()
	if err != nil {
		return stats, err
	}

	return stats, nil
}
