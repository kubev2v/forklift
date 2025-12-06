package settings

import (
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Environment variables.
const (
	LogDevelopment = logging.EnvDevelopment
	LogLevel       = logging.EnvLevel
)

// Logging settings
type Logging struct {
	// Development (mode).
	Development bool
	// Level.
	Level int
}

// Load settings.
func (r *Logging) Load() error {
	r.Development = getEnvBool(LogDevelopment, false)
	r.Level, _ = getPositiveEnvLimit(LogLevel, 0)
	return nil
}
