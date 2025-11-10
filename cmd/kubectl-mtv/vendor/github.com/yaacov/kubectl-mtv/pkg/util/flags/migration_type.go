package flags

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// MigrationTypeFlag implements pflag.Value interface for migration type validation
type MigrationTypeFlag struct {
	value v1beta1.MigrationType
}

func (m *MigrationTypeFlag) String() string {
	return string(m.value)
}

func (m *MigrationTypeFlag) Set(value string) error {
	validTypes := []v1beta1.MigrationType{v1beta1.MigrationCold, v1beta1.MigrationWarm, v1beta1.MigrationLive, v1beta1.MigrationOnlyConversion}

	isValid := false
	for _, validType := range validTypes {
		if v1beta1.MigrationType(value) == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid migration type: %s. Valid types are: cold, warm, live, conversion", value)
	}

	m.value = v1beta1.MigrationType(value)
	return nil
}

func (m *MigrationTypeFlag) Type() string {
	return "string"
}

// GetValue returns the migration type value
func (m *MigrationTypeFlag) GetValue() v1beta1.MigrationType {
	return m.value
}

// GetValidValues returns all valid migration type values for auto-completion
func (m *MigrationTypeFlag) GetValidValues() []string {
	return []string{"cold", "warm", "live", "conversion"}
}

// NewMigrationTypeFlag creates a new migration type flag
func NewMigrationTypeFlag() *MigrationTypeFlag {
	return &MigrationTypeFlag{}
}
