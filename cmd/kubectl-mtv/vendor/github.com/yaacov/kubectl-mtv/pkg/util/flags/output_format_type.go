package flags

import (
	"fmt"
)

// OutputFormatTypeFlag implements pflag.Value interface for output format type validation
type OutputFormatTypeFlag struct {
	value        string
	validFormats []string
}

func (o *OutputFormatTypeFlag) String() string {
	return o.value
}

func (o *OutputFormatTypeFlag) Set(value string) error {
	isValid := false
	for _, validType := range o.validFormats {
		if value == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid output format: %s. Valid formats are: %v", value, o.validFormats)
	}

	o.value = value
	return nil
}

func (o *OutputFormatTypeFlag) Type() string {
	return "string"
}

// GetValue returns the output format type value
func (o *OutputFormatTypeFlag) GetValue() string {
	return o.value
}

// GetValidValues returns all valid output format type values for auto-completion
func (o *OutputFormatTypeFlag) GetValidValues() []string {
	return o.validFormats
}

// NewOutputFormatTypeFlag creates a new output format type flag with standard formats
func NewOutputFormatTypeFlag() *OutputFormatTypeFlag {
	return &OutputFormatTypeFlag{
		validFormats: []string{"table", "json", "yaml"},
		value:        "table", // default value
	}
}
