package flags

import (
	"fmt"
)

// VMInventoryOutputTypeFlag implements pflag.Value interface for VM inventory output format validation
type VMInventoryOutputTypeFlag struct {
	value string
}

func (v *VMInventoryOutputTypeFlag) String() string {
	return v.value
}

func (v *VMInventoryOutputTypeFlag) Set(value string) error {
	validTypes := []string{"table", "json", "yaml", "planvms"}

	isValid := false
	for _, validType := range validTypes {
		if value == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid VM inventory output format: %s. Valid formats are: table, json, yaml, planvms", value)
	}

	v.value = value
	return nil
}

func (v *VMInventoryOutputTypeFlag) Type() string {
	return "string"
}

// GetValue returns the VM inventory output format value
func (v *VMInventoryOutputTypeFlag) GetValue() string {
	return v.value
}

// GetValidValues returns all valid VM inventory output format values for auto-completion
func (v *VMInventoryOutputTypeFlag) GetValidValues() []string {
	return []string{"table", "json", "yaml", "planvms"}
}

// SetDefault sets the default value for the VM inventory output format
func (v *VMInventoryOutputTypeFlag) SetDefault(defaultValue string) {
	v.value = defaultValue
}

// NewVMInventoryOutputTypeFlag creates a new VM inventory output format type flag
func NewVMInventoryOutputTypeFlag() *VMInventoryOutputTypeFlag {
	return &VMInventoryOutputTypeFlag{
		value: "table", // default value
	}
}
