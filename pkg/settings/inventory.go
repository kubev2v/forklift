package settings

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// k8s pod default.
const (
	ServiceCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"
)

// Environment variables.
const (
	AllowedOrigins = "CORS_ALLOWED_ORIGINS"
	WorkingDir     = "WORKING_DIR"
	AuthRequired   = "AUTH_REQUIRED"
	Host           = "API_HOST"
	Namespace      = "POD_NAMESPACE"
	Port           = "API_PORT"
	TLSCertificate = "API_TLS_CERTIFICATE"
	TLSKey         = "API_TLS_KEY"
	TLSCa          = "API_TLS_CA"
)

// CORS
type CORS struct {
	// Allowed origins.
	AllowedOrigins []string
}

// Inventory settings.
type Inventory struct {
	// CORS settings.
	CORS CORS
	// DB working directory.
	WorkingDir string
	// Authorization required.
	AuthRequired bool
	// Host.
	Host string
	// Pod namespace
	Namespace string
	// Port
	Port int
	// TLS
	TLS struct {
		// Certificate path
		Certificate string
		// Key path
		Key string
		// CA path
		CA string
	}
}

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
	r.AuthRequired = getEnvBool(AuthRequired, true)
	// Host
	if s, found := os.LookupEnv(Host); found {
		r.Host = s
	} else {
		r.Host = "localhost"
	}
	// Namespace
	if s, found := os.LookupEnv(Namespace); found {
		r.Namespace = s
	}
	// Port
	if s, found := os.LookupEnv(Port); found {
		r.Port, _ = strconv.Atoi(s)
	} else {
		r.Port = 8080
	}
	// TLS
	if s, found := os.LookupEnv(TLSCertificate); found {
		r.TLS.Certificate = s
	}
	if s, found := os.LookupEnv(TLSKey); found {
		r.TLS.Key = s
	}
	if s, found := os.LookupEnv(TLSCa); found {
		r.TLS.CA = s
	} else {
		if _, err := os.Stat(ServiceCAFile); !errors.Is(err, os.ErrNotExist) {
			r.TLS.CA = ServiceCAFile
		}
	}

	return nil
}
