package settings

import (
	"fmt"
	"os"
	"strconv"
)

// Environment variables.
const (
	MetricsPort = "METRICS_PORT"
)

// Metrics settings
type Metrics struct {
	// Metrics port. 0 = disabled.
	Port int
}

// Load settings.
func (r *Metrics) Load() error {
	// Port
	if s, found := os.LookupEnv(MetricsPort); found {
		r.Port, _ = strconv.Atoi(s)
	} else {
		r.Port = 8080
	}

	return nil
}

// Metrics address.
// Port = 0 will disable metrics.
func (r *Metrics) Address() string {
	if r.Port > 0 {
		return fmt.Sprintf(":%d", r.Port)
	} else {
		return "0"
	}
}
