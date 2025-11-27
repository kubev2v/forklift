package dynamic

import (
	"reflect"

	"github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
)

// vmChanged compares two VM models to determine if an update is needed.
// It performs deep JSON comparison to detect all changes including nested fields,
// and logs which typed fields changed for debugging.
func (r *Collector) vmChanged(old, new *dynamic.VM) bool {
	// Parse full JSON objects
	oldObj, err := old.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse old object JSON",
			"error", err,
			"id", old.ID)
		// Can't compare, assume changed to be safe
		return true
	}

	newObj, err := new.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse new object JSON",
			"error", err,
			"id", new.ID)
		// Can't compare, assume changed to be safe
		return true
	}

	// Deep compare entire JSON structure (handles nested arrays/objects)
	changed := !reflect.DeepEqual(oldObj, newObj)

	// Log which typed fields changed for debugging
	if changed {
		if old.Name != new.Name {
			log.V(4).Info("Name changed",
				"vm", old.Name,
				"old", old.Name,
				"new", new.Name)
		}
		if old.CPUs != new.CPUs {
			log.V(4).Info("CPUs changed",
				"vm", old.Name,
				"old", old.CPUs,
				"new", new.CPUs)
		}
		if old.Memory != new.Memory {
			log.V(4).Info("Memory changed",
				"vm", old.Name,
				"old", old.Memory,
				"new", new.Memory)
		}
		if old.PowerState != new.PowerState {
			log.V(4).Info("PowerState changed",
				"vm", old.Name,
				"old", old.PowerState,
				"new", new.PowerState)
		}
	}

	return changed
}

// networkChanged compares two Network models to determine if an update is needed.
// It performs deep JSON comparison to detect all changes including nested fields,
// and logs which typed fields changed for debugging.
func (r *Collector) networkChanged(old, new *dynamic.Network) bool {
	// Parse full JSON objects
	oldObj, err := old.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse old network JSON",
			"error", err,
			"id", old.ID)
		return true
	}

	newObj, err := new.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse new network JSON",
			"error", err,
			"id", new.ID)
		return true
	}

	// Deep compare entire JSON structure
	changed := !reflect.DeepEqual(oldObj, newObj)

	// Log which typed fields changed for debugging
	if changed && old.Name != new.Name {
		log.V(4).Info("Network name changed",
			"network", old.Name,
			"old", old.Name,
			"new", new.Name)
	}

	return changed
}

// storageChanged compares two Storage models to determine if an update is needed.
// It performs deep JSON comparison to detect all changes including nested fields,
// and logs which typed fields changed for debugging.
func (r *Collector) storageChanged(old, new *dynamic.Storage) bool {
	// Parse full JSON objects
	oldObj, err := old.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse old storage JSON",
			"error", err,
			"id", old.ID)
		return true
	}

	newObj, err := new.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse new storage JSON",
			"error", err,
			"id", new.ID)
		return true
	}

	// Deep compare entire JSON structure
	changed := !reflect.DeepEqual(oldObj, newObj)

	// Log which typed fields changed for debugging
	if changed {
		if old.Name != new.Name {
			log.V(4).Info("Storage name changed",
				"storage", old.Name,
				"old", old.Name,
				"new", new.Name)
		}
		if old.Capacity != new.Capacity {
			log.V(4).Info("Storage capacity changed",
				"storage", old.Name,
				"old", old.Capacity,
				"new", new.Capacity)
		}
	}

	return changed
}

// diskChanged compares two Disk models to determine if an update is needed.
// It performs deep JSON comparison to detect all changes including nested fields,
// and logs which typed fields changed for debugging.
func (r *Collector) diskChanged(old, new *dynamic.Disk) bool {
	// Parse full JSON objects
	oldObj, err := old.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse old disk JSON",
			"error", err,
			"id", old.ID)
		return true
	}

	newObj, err := new.GetObject()
	if err != nil {
		log.V(3).Info("Failed to parse new disk JSON",
			"error", err,
			"id", new.ID)
		return true
	}

	// Deep compare entire JSON structure
	changed := !reflect.DeepEqual(oldObj, newObj)

	// Log which typed fields changed for debugging
	if changed {
		if old.Name != new.Name {
			log.V(4).Info("Disk name changed",
				"disk", old.Name,
				"old", old.Name,
				"new", new.Name)
		}
		if old.Capacity != new.Capacity {
			log.V(4).Info("Disk capacity changed",
				"disk", old.Name,
				"old", old.Capacity,
				"new", new.Capacity)
		}
		if old.Shared != new.Shared {
			log.V(4).Info("Disk shared status changed",
				"disk", old.Name,
				"old", old.Shared,
				"new", new.Shared)
		}
	}

	return changed
}
