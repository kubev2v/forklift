package settings

import (
	"os"
	"strings"
)

//
// Environment variables.
const (
	AllowedOrigins = "CORS_ALLOWED_ORIGINS"
	WorkingDir     = "WORKING_DIR"
	AuthOptional   = "AUTH_OPTIONAL"
)

//
// CORS
type CORS struct {
	// Allowed origins.
	AllowedOrigins []string
}

//
// Inventory settings.
type Inventory struct {
	// CORS settings.
	CORS CORS
	// DB working directory.
	WorkingDir string
	// Authorization disabled.
	AuthOptional bool
}

//
// Load settings.
func (r *Inventory) Load() error {
	r.CORS = CORS{
		AllowedOrigins: []string{},
	}
	// AllowedOrigins
	if s, found := os.LookupEnv(AllowedOrigins); found {
		parts := strings.Fields(s)
		for _, s := range parts {
			if len(s) > 0 {
				r.CORS.AllowedOrigins = append(r.CORS.AllowedOrigins, s)
			}
		}
	}
	// WorkingDir
	if s, found := os.LookupEnv(WorkingDir); found {
		r.WorkingDir = s
	} else {
		r.WorkingDir = os.TempDir()
	}
	// Auth
	r.AuthOptional = getEnvBool(AuthOptional, false)

	return nil
}
