package dynamic

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
)

// beginWatch initiates watches on all inventory resource types in the SQLite cache.
// This allows the collector to fire events when resources are created, updated, or deleted.
func (r *Collector) beginWatch() (err error) {
	defer func() {
		if err != nil {
			r.endWatch()
		}
	}()

	// Watch VM changes
	vmWatch, err := r.db.Watch(
		&dynamic.VM{},
		&VMEventHandler{
			Provider: r.provider,
			log:      log,
		})
	if err == nil {
		r.watches = append(r.watches, vmWatch)
	} else {
		return
	}

	// Watch Network changes
	networkWatch, err := r.db.Watch(
		&dynamic.Network{},
		&NetworkEventHandler{
			Provider: r.provider,
			log:      log,
		})
	if err == nil {
		r.watches = append(r.watches, networkWatch)
	} else {
		return
	}

	// Watch Storage changes
	storageWatch, err := r.db.Watch(
		&dynamic.Storage{},
		&StorageEventHandler{
			Provider: r.provider,
			log:      log,
		})
	if err == nil {
		r.watches = append(r.watches, storageWatch)
	} else {
		return
	}

	// Watch Disk changes
	diskWatch, err := r.db.Watch(
		&dynamic.Disk{},
		&DiskEventHandler{
			Provider: r.provider,
			log:      log,
		})
	if err == nil {
		r.watches = append(r.watches, diskWatch)
	} else {
		return
	}

	log.V(3).Info("Watches initialized",
		"provider", r.provider.Name)
	return nil
}

// endWatch terminates all active watches and clears the watch list.
func (r *Collector) endWatch() {
	for _, watch := range r.watches {
		watch.End()
	}
	r.watches = nil
}
