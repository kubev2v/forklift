package flags

import (
	"fmt"
)

// EsxiCloneMethodFlag implements pflag.Value interface for ESXi clone method validation
type EsxiCloneMethodFlag struct {
	value string
}

func (e *EsxiCloneMethodFlag) String() string {
	return e.value
}

func (e *EsxiCloneMethodFlag) Set(value string) error {
	validMethods := []string{"vib", "ssh"}

	isValid := false
	for _, m := range validMethods {
		if value == m {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid ESXi clone method: %s. Valid methods are: vib, ssh", value)
	}

	e.value = value
	return nil
}

func (e *EsxiCloneMethodFlag) Type() string {
	return "string"
}

// GetValue returns the ESXi clone method value
func (e *EsxiCloneMethodFlag) GetValue() string {
	return e.value
}

// GetValidValues returns all valid ESXi clone method values for auto-completion
func (e *EsxiCloneMethodFlag) GetValidValues() []string {
	return []string{"vib", "ssh"}
}

// NewEsxiCloneMethodFlag creates a new ESXi clone method flag
func NewEsxiCloneMethodFlag() *EsxiCloneMethodFlag {
	return &EsxiCloneMethodFlag{}
}
