package flags

import (
	"fmt"
)

// MappingTypeFlag implements pflag.Value interface for mapping type validation
type MappingTypeFlag struct {
	value string
}

func (m *MappingTypeFlag) String() string {
	return m.value
}

func (m *MappingTypeFlag) Set(value string) error {
	validTypes := []string{"network", "storage"}

	isValid := false
	for _, validType := range validTypes {
		if value == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid mapping type: %s. Valid types are: network, storage", value)
	}

	m.value = value
	return nil
}

func (m *MappingTypeFlag) Type() string {
	return "string"
}

// GetValue returns the mapping type value
func (m *MappingTypeFlag) GetValue() string {
	return m.value
}

// GetValidValues returns all valid mapping type values for auto-completion
func (m *MappingTypeFlag) GetValidValues() []string {
	return []string{"network", "storage"}
}

// NewMappingTypeFlag creates a new mapping type flag with default value "network"
func NewMappingTypeFlag() *MappingTypeFlag {
	return &MappingTypeFlag{
		value: "network", // Default value set here
	}
}
