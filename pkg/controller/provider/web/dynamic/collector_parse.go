package dynamic

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// parseVM extracts common fields from JSON and creates a hybrid VM model.
// Typed fields (CPUs, Memory, PowerState) are extracted for change detection,
// while the complete JSON is stored in the Object field.
func (r *Collector) parseVM(data map[string]interface{}) (*dynamic.VM, error) {
	vm := &dynamic.VM{}

	// Required field: id
	id, ok := data["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' field")
	}
	vm.ID = id

	// Required field: name
	name, ok := data["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'name' field")
	}
	vm.Name = name

	// Optional fields for change detection
	if cpuCount, ok := data["cpuCount"].(float64); ok {
		vm.CPUs = int32(cpuCount)
	}
	if memoryMB, ok := data["memoryMB"].(float64); ok {
		vm.Memory = int64(memoryMB)
	}
	if powerState, ok := data["powerState"].(string); ok {
		vm.PowerState = powerState
	}

	// Store complete JSON
	err := vm.SetObject(data)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to serialize object")
	}

	return vm, nil
}

// parseNetwork extracts common fields from JSON and creates a hybrid Network model.
// The complete JSON is stored in the Object field.
func (r *Collector) parseNetwork(data map[string]interface{}) (*dynamic.Network, error) {
	network := &dynamic.Network{}

	// Required field: id
	id, ok := data["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' field")
	}
	network.ID = id

	// Required field: name
	name, ok := data["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'name' field")
	}
	network.Name = name

	// Store complete JSON
	err := network.SetObject(data)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to serialize object")
	}

	return network, nil
}

// parseStorage extracts common fields from JSON and creates a hybrid Storage model.
// Typed fields (Capacity) are extracted for change detection,
// while the complete JSON is stored in the Object field.
func (r *Collector) parseStorage(data map[string]interface{}) (*dynamic.Storage, error) {
	storage := &dynamic.Storage{}

	// Required field: id
	id, ok := data["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' field")
	}
	storage.ID = id

	// Required field: name
	name, ok := data["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'name' field")
	}
	storage.Name = name

	// Optional field: capacityBytes for change detection
	if capacityBytes, ok := data["capacityBytes"].(float64); ok {
		storage.Capacity = int64(capacityBytes)
	}

	// Store complete JSON
	err := storage.SetObject(data)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to serialize object")
	}

	return storage, nil
}

// parseDisk extracts common fields from JSON and creates a hybrid Disk model.
// Typed fields (Capacity, Shared) are extracted for change detection,
// while the complete JSON is stored in the Object field.
func (r *Collector) parseDisk(data map[string]interface{}) (*dynamic.Disk, error) {
	disk := &dynamic.Disk{}

	// Required field: id
	id, ok := data["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' field")
	}
	disk.ID = id

	// Required field: name
	name, ok := data["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'name' field")
	}
	disk.Name = name

	// Optional field: capacityBytes for change detection
	if capacityBytes, ok := data["capacityBytes"].(float64); ok {
		disk.Capacity = int64(capacityBytes)
	}

	// Optional field: shared for change detection
	if shared, ok := data["shared"].(bool); ok {
		disk.Shared = shared
	}

	// Store complete JSON
	err := disk.SetObject(data)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to serialize object")
	}

	return disk, nil
}
