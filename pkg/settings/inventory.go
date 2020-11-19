package settings

import (
	"os"
	"strconv"
	"strings"
)

//
// Environment variables.
const (
	AllowedOrigins = "CORS_ALLOWED_ORIGINS"
	WorkingDir     = "WORKING_DIR"
	AuthOptional   = "AUTH_OPTIONAL"
	Host           = "API_HOST"
	Port           = "API_PORT"
	TLSEnabled     = "API_TLS_ENABLED"
	TLSCertificate = "API_TLS_CERTIFICATE"
	TLSKey         = "API_TLS_KEY"
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
	// Host.
	Host string
	// Port
	Port int
	// TLS
	TLS struct {
		// Enabled.
		Enabled bool
		// Certificate path
		Certificate string
		// Key path
		Key string
	}
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
	// Host
	if s, found := os.LookupEnv(Host); found {
		r.Host = s
	} else {
		r.Host = "localhost"
	}
	// Port
	if s, found := os.LookupEnv(Port); found {
		r.Port, _ = strconv.Atoi(s)
	} else {
		r.Port = 8080
	}
	// TLS
	r.TLS.Enabled = false
	if s, found := os.LookupEnv(TLSEnabled); found {
		if r.TLS.Enabled, _ = strconv.ParseBool(s); r.TLS.Enabled {
			if cert, found := os.LookupEnv(TLSCertificate); found {
				if key, found := os.LookupEnv(TLSKey); found {
					r.TLS.Certificate = cert
					r.TLS.Key = key
				}
			}
		}
	}

	return nil
}
