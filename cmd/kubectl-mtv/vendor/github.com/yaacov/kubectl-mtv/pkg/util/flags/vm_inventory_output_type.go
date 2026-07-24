package flags

import (
	"fmt"
	"strings"
)

// vmInventoryOutputFormats is the single source of truth for valid VM inventory output formats.
var vmInventoryOutputFormats = []string{"table", "json", "yaml", "markdown", "planvms"}

// VMInventoryOutputTypeFlag implements pflag.Value interface for VM inventory output format validation
type VMInventoryOutputTypeFlag struct {
	value string
}

func (v *VMInventoryOutputTypeFlag) String() string {
	return v.value
}

func (v *VMInventoryOutputTypeFlag) Set(value string) error {
	for _, valid := range vmInventoryOutputFormats {
		if value == valid {
			v.value = value
			return nil
		}
	}
	return fmt.Errorf("invalid VM inventory output format: %s. Valid formats are: %s", value, strings.Join(vmInventoryOutputFormats, ", "))
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
	return vmInventoryOutputFormats
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
