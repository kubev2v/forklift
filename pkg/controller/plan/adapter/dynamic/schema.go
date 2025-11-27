package dynamic

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// ProviderSchema defines the complete schema for a dynamic provider.
// It maps JSON field paths to semantic meanings.
type ProviderSchema struct {
	VM      VMSchema
	Disk    DiskSchema
	Network NetworkSchema
	NIC     NICSchema
	Storage StorageSchema
}

// VMSchema defines field mappings for VM objects.
type VMSchema struct {
	// Required fields that MUST exist
	Required map[string]string // field name → JSONPath
	// Optional fields with JSONPath mappings
	Fields map[string]string // field name → JSONPath
}

// DiskSchema defines field mappings for Disk objects.
type DiskSchema struct {
	Required map[string]string
	Fields   map[string]string
}

// NetworkSchema defines field mappings for Network objects.
type NetworkSchema struct {
	Required map[string]string
	Fields   map[string]string
}

// NICSchema defines field mappings for NIC objects.
type NICSchema struct {
	Required map[string]string
	Fields   map[string]string
}

// StorageSchema defines field mappings for Storage objects.
type StorageSchema struct {
	Required map[string]string
	Fields   map[string]string
}

// LoadSchema loads schema from Provider CRD or returns default schema.
// TODO: Read from provider.Spec.Schema when CRD is updated
func LoadSchema(provider *api.Provider) (*ProviderSchema, error) {
	// For now, return default schema based on provider type
	// TODO: Parse schema from provider.Spec.Schema

	return DefaultSchema(provider.Type()), nil
}

// DefaultSchema provides default schemas for known provider types.
// This serves as both a fallback and documentation of the expected schema.
func DefaultSchema(providerType api.ProviderType) *ProviderSchema {
	// All dynamic providers use the generic schema
	// Provider-specific schemas can be added here if needed
	return GenericSchema()
}

// OvaDefaultSchema returns the default schema for OVA providers.
func OvaDefaultSchema() *ProviderSchema {
	return &ProviderSchema{
		VM: VMSchema{
			Required: map[string]string{
				"id":   "id",
				"name": "name",
			},
			Fields: map[string]string{
				// Identification
				"uuid": "uuid",
				"path": "path",

				// CPU and Memory
				"cpuCount":       "cpuCount",
				"coresPerSocket": "coresPerSocket",
				"memoryMB":       "memoryMB",
				"memoryUnits":    "memoryUnits",

				// Firmware
				"firmware":   "firmware",
				"secureBoot": "secureBoot",

				// Collections
				"disks":    "disks",
				"nics":     "nics",
				"networks": "networks",

				// Validation
				"concerns": "concerns",
			},
		},
		Disk: DiskSchema{
			Required: map[string]string{
				"id":       "id",
				"capacity": "capacity",
			},
			Fields: map[string]string{
				"name":                    "name",
				"filePath":                "filePath",
				"path":                    "path",
				"capacityAllocationUnits": "capacityAllocationUnits",
			},
		},
		Network: NetworkSchema{
			Required: map[string]string{
				"id":   "id",
				"name": "name",
			},
			Fields: map[string]string{
				"path": "path",
			},
		},
		NIC: NICSchema{
			Required: map[string]string{
				"id": "id",
			},
			Fields: map[string]string{
				"name":    "name",
				"network": "network",
				"mac":     "mac",
			},
		},
		Storage: StorageSchema{
			Required: map[string]string{
				"id":   "id",
				"name": "name",
			},
			Fields: map[string]string{},
		},
	}
}

// GenericSchema returns a generic schema that works with most providers.
// This is the absolute minimum contract required.
func GenericSchema() *ProviderSchema {
	return &ProviderSchema{
		VM: VMSchema{
			Required: map[string]string{
				"id":   "id",
				"name": "name",
			},
			Fields: map[string]string{
				"cpuCount": "cpuCount",
				"memoryMB": "memoryMB",
				"disks":    "disks",
				"networks": "networks",
			},
		},
		Disk: DiskSchema{
			Required: map[string]string{
				"id":       "id",
				"capacity": "capacity",
			},
			Fields: map[string]string{},
		},
		Network: NetworkSchema{
			Required: map[string]string{
				"id":   "id",
				"name": "name",
			},
			Fields: map[string]string{},
		},
		NIC: NICSchema{
			Required: map[string]string{
				"id": "id",
			},
			Fields: map[string]string{
				"network": "network",
			},
		},
		Storage: StorageSchema{
			Required: map[string]string{
				"id":   "id",
				"name": "name",
			},
			Fields: map[string]string{},
		},
	}
}

// GetField retrieves a field value from the schema mapping.
// Returns the JSONPath and whether it was found.
func (s *VMSchema) GetField(fieldName string) (jsonPath string, found bool) {
	// Check required first
	if path, ok := s.Required[fieldName]; ok {
		return path, true
	}
	// Check optional fields
	if path, ok := s.Fields[fieldName]; ok {
		return path, true
	}
	return "", false
}

// GetField retrieves a field value from the disk schema mapping.
func (s *DiskSchema) GetField(fieldName string) (jsonPath string, found bool) {
	if path, ok := s.Required[fieldName]; ok {
		return path, true
	}
	if path, ok := s.Fields[fieldName]; ok {
		return path, true
	}
	return "", false
}

// GetField retrieves a field value from the network schema mapping.
func (s *NetworkSchema) GetField(fieldName string) (jsonPath string, found bool) {
	if path, ok := s.Required[fieldName]; ok {
		return path, true
	}
	if path, ok := s.Fields[fieldName]; ok {
		return path, true
	}
	return "", false
}

// GetField retrieves a field value from the NIC schema mapping.
func (s *NICSchema) GetField(fieldName string) (jsonPath string, found bool) {
	if path, ok := s.Required[fieldName]; ok {
		return path, true
	}
	if path, ok := s.Fields[fieldName]; ok {
		return path, true
	}
	return "", false
}

// GetField retrieves a field value from the storage schema mapping.
func (s *StorageSchema) GetField(fieldName string) (jsonPath string, found bool) {
	if path, ok := s.Required[fieldName]; ok {
		return path, true
	}
	if path, ok := s.Fields[fieldName]; ok {
		return path, true
	}
	return "", false
}

// ValidateMinimumContract verifies the schema meets minimum requirements.
func (s *ProviderSchema) ValidateMinimumContract() error {
	// VM must have id and name
	if _, ok := s.VM.Required["id"]; !ok {
		return fmt.Errorf("VM schema missing required field: id")
	}
	if _, ok := s.VM.Required["name"]; !ok {
		return fmt.Errorf("VM schema missing required field: name")
	}

	// Disk must have id and capacity
	if _, ok := s.Disk.Required["id"]; !ok {
		return fmt.Errorf("Disk schema missing required field: id")
	}
	if _, ok := s.Disk.Required["capacity"]; !ok {
		return fmt.Errorf("Disk schema missing required field: capacity")
	}

	// Network must have id and name
	if _, ok := s.Network.Required["id"]; !ok {
		return fmt.Errorf("Network schema missing required field: id")
	}
	if _, ok := s.Network.Required["name"]; !ok {
		return fmt.Errorf("Network schema missing required field: name")
	}

	return nil
}
