package flags

import (
	"fmt"
)

// SdkEndpointTypeFlag implements pflag.Value interface for SDK endpoint type validation
type SdkEndpointTypeFlag struct {
	value string
}

func (s *SdkEndpointTypeFlag) String() string {
	return s.value
}

func (s *SdkEndpointTypeFlag) Set(value string) error {
	validTypes := []string{"vcenter", "esxi"}

	isValid := false
	for _, validType := range validTypes {
		if value == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid SDK endpoint type: %s. Valid types are: vcenter, esxi", value)
	}

	s.value = value
	return nil
}

func (s *SdkEndpointTypeFlag) Type() string {
	return "string"
}

// GetValue returns the SDK endpoint type value
func (s *SdkEndpointTypeFlag) GetValue() string {
	return s.value
}

// GetValidValues returns all valid SDK endpoint type values for auto-completion
func (s *SdkEndpointTypeFlag) GetValidValues() []string {
	return []string{"vcenter", "esxi"}
}

// NewSdkEndpointTypeFlag creates a new SDK endpoint type flag
func NewSdkEndpointTypeFlag() *SdkEndpointTypeFlag {
	return &SdkEndpointTypeFlag{}
}
