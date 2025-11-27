package dynamic

import (
	"encoding/json"
	"fmt"

	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Base type with common required fields for all resources
type Resource struct {
	// Resource ID (primary key)
	ID string `sql:"pk" json:"id"`
	// Resource name (required by all providers)
	Name string `sql:"d0" json:"name"`
	// Complete resource data as JSON
	Object string `sql:"d1" json:"object"`
}

// VM represents a virtual machine with common fields + JSON blob
type VM struct {
	Resource
	// CPU count (for change detection)
	CPUs int32 `sql:"d2" json:"cpus,omitempty"`
	// Memory in MB (for change detection)
	Memory int64 `sql:"d3" json:"memory,omitempty"`
	// Power state (for change detection)
	PowerState string `sql:"d4" json:"powerState,omitempty"`
}

func (m *VM) Pk() string {
	return m.ID
}

func (m *VM) String() string {
	return m.Name
}

func (m *VM) Labels() libmodel.Labels {
	return nil
}

// GetObject returns the complete data as map
func (m *VM) GetObject() (map[string]interface{}, error) {
	if m.Object == "" {
		return nil, fmt.Errorf("VM %s (id=%s) has empty object field", m.Name, m.ID)
	}
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(m.Object), &obj)
	return obj, err
}

// SetObject stores complete data as JSON
func (m *VM) SetObject(obj map[string]interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	m.Object = string(data)
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling for REST API responses
// This converts raw provider JSON objects into model structs
func (m *VM) UnmarshalJSON(data []byte) error {
	// Unmarshal into a generic map to extract fields
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// Extract required fields
	if id, ok := obj["id"].(string); ok {
		m.ID = id
	}
	if name, ok := obj["name"].(string); ok {
		m.Name = name
	}

	// Extract change-detection fields (optional)
	if cpus, ok := obj["cpus"].(float64); ok {
		m.CPUs = int32(cpus)
	}
	if memory, ok := obj["memory"].(float64); ok {
		m.Memory = int64(memory)
	}
	if powerState, ok := obj["powerState"].(string); ok {
		m.PowerState = powerState
	}

	// Store the complete object as JSON string
	m.Object = string(data)
	return nil
}

// Network represents a network with common fields + JSON blob
type Network struct {
	Resource
}

func (m *Network) Pk() string {
	return m.ID
}

func (m *Network) String() string {
	return m.Name
}

func (m *Network) Labels() libmodel.Labels {
	return nil
}

func (m *Network) GetObject() (map[string]interface{}, error) {
	if m.Object == "" {
		return nil, fmt.Errorf("Network %s (id=%s) has empty object field", m.Name, m.ID)
	}
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(m.Object), &obj)
	return obj, err
}

func (m *Network) SetObject(obj map[string]interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	m.Object = string(data)
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling for REST API responses
func (m *Network) UnmarshalJSON(data []byte) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// Extract required fields
	if id, ok := obj["id"].(string); ok {
		m.ID = id
	}
	if name, ok := obj["name"].(string); ok {
		m.Name = name
	}

	// Store the complete object as JSON string
	m.Object = string(data)
	return nil
}

// Storage represents a storage resource with common fields + JSON blob
type Storage struct {
	Resource
	// Capacity in bytes (for change detection)
	Capacity int64 `sql:"d2" json:"capacity,omitempty"`
}

func (m *Storage) Pk() string {
	return m.ID
}

func (m *Storage) String() string {
	return m.Name
}

func (m *Storage) Labels() libmodel.Labels {
	return nil
}

func (m *Storage) GetObject() (map[string]interface{}, error) {
	if m.Object == "" {
		return nil, fmt.Errorf("Storage %s (id=%s) has empty object field", m.Name, m.ID)
	}
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(m.Object), &obj)
	return obj, err
}

func (m *Storage) SetObject(obj map[string]interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	m.Object = string(data)
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling for REST API responses
func (m *Storage) UnmarshalJSON(data []byte) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// Extract required fields
	if id, ok := obj["id"].(string); ok {
		m.ID = id
	}
	if name, ok := obj["name"].(string); ok {
		m.Name = name
	}

	// Extract change-detection fields (optional)
	if capacity, ok := obj["capacity"].(float64); ok {
		m.Capacity = int64(capacity)
	}

	// Store the complete object as JSON string
	m.Object = string(data)
	return nil
}

// Disk represents a disk resource with common fields + JSON blob
type Disk struct {
	Resource
	// Capacity in bytes (for change detection)
	Capacity int64 `sql:"d2" json:"capacity,omitempty"`
	// Shared flag (for change detection)
	Shared bool `sql:"d3" json:"shared,omitempty"`
}

func (m *Disk) Pk() string {
	return m.ID
}

func (m *Disk) String() string {
	return m.Name
}

func (m *Disk) Labels() libmodel.Labels {
	return nil
}

func (m *Disk) GetObject() (map[string]interface{}, error) {
	if m.Object == "" {
		return nil, fmt.Errorf("Disk %s (id=%s) has empty object field", m.Name, m.ID)
	}
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(m.Object), &obj)
	return obj, err
}

func (m *Disk) SetObject(obj map[string]interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	m.Object = string(data)
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling for REST API responses
func (m *Disk) UnmarshalJSON(data []byte) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// Extract required fields
	if id, ok := obj["id"].(string); ok {
		m.ID = id
	}
	if name, ok := obj["name"].(string); ok {
		m.Name = name
	}

	// Extract change-detection fields (optional)
	if capacity, ok := obj["capacity"].(float64); ok {
		m.Capacity = int64(capacity)
	}
	if shared, ok := obj["shared"].(bool); ok {
		m.Shared = shared
	}

	// Store the complete object as JSON string
	m.Object = string(data)
	return nil
}

// Workload is an alias for VM (for compatibility)
type Workload = VM
