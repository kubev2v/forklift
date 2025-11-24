package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// load orchestrates the initial inventory load from the dynamic provider.
// It fetches VMs, networks, storage, and disks and inserts them into the SQLite cache.
func (r *Collector) load(ctx context.Context) (err error) {
	if r.config == nil {
		return fmt.Errorf("no config found")
	}

	mark := time.Now()

	// Load VMs
	vmsCount, err := r.loadVMs()
	if err != nil {
		return err
	}

	// Load Networks
	networksCount, err := r.loadNetworks()
	if err != nil {
		return err
	}

	// Load Storage
	storageCount, err := r.loadStorage()
	if err != nil {
		return err
	}

	// Load Disks
	disksCount, err := r.loadDisks()
	if err != nil {
		return err
	}

	log.Info("Initial inventory load complete",
		"provider", r.provider.Name,
		"vms", vmsCount,
		"networks", networksCount,
		"storage", storageCount,
		"disks", disksCount,
		"duration", time.Since(mark))

	return nil
}

// loadVMs fetches VMs from the external provider and inserts them into the cache.
func (r *Collector) loadVMs() (int, error) {
	vmsURL := r.config.ServiceURL + "/vms"
	resp, err := http.Get(vmsURL)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to fetch VMs")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to read response")
	}

	var vmsData []map[string]interface{}
	err = json.Unmarshal(body, &vmsData)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to parse VMs JSON")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.End()
	}()

	for _, vmData := range vmsData {
		vm, err := r.parseVM(vmData)
		if err != nil {
			log.V(3).Info("Failed to parse VM, skipping",
				"error", err,
				"data", vmData)
			continue
		}

		err = tx.Insert(vm)
		if err != nil {
			return 0, liberr.Wrap(err, "failed to insert VM")
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(vmsData), nil
}

// loadNetworks fetches networks from the external provider and inserts them into the cache.
func (r *Collector) loadNetworks() (int, error) {
	networksURL := r.config.ServiceURL + "/networks"
	resp, err := http.Get(networksURL)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to fetch networks")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to read response")
	}

	var networksData []map[string]interface{}
	err = json.Unmarshal(body, &networksData)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to parse networks JSON")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.End()
	}()

	for _, networkData := range networksData {
		network, err := r.parseNetwork(networkData)
		if err != nil {
			log.V(3).Info("Failed to parse network, skipping",
				"error", err,
				"data", networkData)
			continue
		}

		err = tx.Insert(network)
		if err != nil {
			return 0, liberr.Wrap(err, "failed to insert network")
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(networksData), nil
}

// loadStorage fetches storage from the external provider and inserts it into the cache.
func (r *Collector) loadStorage() (int, error) {
	storageURL := r.config.ServiceURL + "/storages"
	resp, err := http.Get(storageURL)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to fetch storage")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to read response")
	}

	var storageData []map[string]interface{}
	err = json.Unmarshal(body, &storageData)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to parse storage JSON")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.End()
	}()

	for _, storData := range storageData {
		storage, err := r.parseStorage(storData)
		if err != nil {
			log.V(3).Info("Failed to parse storage, skipping",
				"error", err,
				"data", storData)
			continue
		}

		err = tx.Insert(storage)
		if err != nil {
			return 0, liberr.Wrap(err, "failed to insert storage")
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(storageData), nil
}

// loadDisks fetches disks from the external provider and inserts them into the cache.
func (r *Collector) loadDisks() (int, error) {
	disksURL := r.config.ServiceURL + "/disks"
	resp, err := http.Get(disksURL)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to fetch disks")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, liberr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to read response")
	}

	var disksData []map[string]interface{}
	err = json.Unmarshal(body, &disksData)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to parse disks JSON")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.End()
	}()

	for _, diskData := range disksData {
		disk, err := r.parseDisk(diskData)
		if err != nil {
			log.V(3).Info("Failed to parse disk, skipping",
				"error", err,
				"data", diskData)
			continue
		}

		err = tx.Insert(disk)
		if err != nil {
			return 0, liberr.Wrap(err, "failed to insert disk")
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(disksData), nil
}
