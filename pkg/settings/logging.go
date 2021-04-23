package settings

import (
	"github.com/konveyor/controller/pkg/logging"
)

//
// Environment variables.
const (
	LogDevelopment = logging.EnvDevelopment
	LogLevel       = logging.EnvLevel
)

//
// Logging settings
type Logging struct {
	// Development (mode).
	Development bool
	// Level.
	Level int
}

//
// Load settings.
func (r *Logging) Load() error {
	r.Development = getEnvBool(LogDevelopment, false)
	r.Level, _ = getEnvLimit(LogLevel, 0)
	return nil
}
