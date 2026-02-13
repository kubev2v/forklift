package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Environment variables.
const (
	EnvApplianceEndpoints = "APPLIANCE_ENDPOINTS"
	EnvAuthRequired       = "AUTH_REQUIRED"
	EnvCatalogPath        = "CATALOG_PATH"
	EnvPort               = "PORT"
	EnvProviderNamespace  = "PROVIDER_NAMESPACE"
	EnvProviderName       = "PROVIDER_NAME"
	EnvProviderVerb       = "PROVIDER_VERB"
	EnvTokenCacheTTL      = "TOKEN_CACHE_TTL"
	// HyperV-specific environment variables
	EnvHyperVUrl       = "HYPERV_URL"
	EnvHyperVUsername  = "HYPERV_USERNAME"
	EnvHyperVPassword  = "HYPERV_PASSWORD"
	EnvSMBUrl          = "SMB_URL"
	EnvRefreshInterval = "REFRESH_INTERVAL"
)

const (
	SecretMountPath = "/etc/secret"
)

// Default values.
const (
	DefaultRefreshInterval = 5 * time.Second
)

// ProviderSettings contains common settings for OVF-based provider servers (OVA, HyperV).
type ProviderSettings struct {
	// Whether the appliance management endpoints are enabled
	ApplianceEndpoints bool
	Auth               struct {
		// Whether (k8s) auth is required. If true,
		// the user's token must have access to the related
		// provider CR.
		Required bool
		// How long to cache a valid token review (seconds)
		TTL int
	}
	// Path to appliance directory (OVA catalog or HyperV SMB mount)
	CatalogPath string
	// Port to serve on
	Port string
	// Provider details
	Provider struct {
		Name      string
		Namespace string
		Verb      string
	}
	// Default catalog path if not specified via environment
	DefaultCatalogPath string
	// HyperV-specific settings (libvirt-based provider)
	HyperV HyperVSettings
}

type HyperVSettings struct {
	URL                string
	Username           string
	Password           string
	SMBUrl             string
	RefreshInterval    time.Duration
	InsecureSkipVerify bool   // Read from /etc/secret/insecureSkipVerify
	CACertPath         string // Path to /etc/secret/cacert if exists
}

// Load loads settings from environment variables.
func (r *ProviderSettings) Load() (err error) {
	r.ApplianceEndpoints = getEnvBool(EnvApplianceEndpoints, false)
	r.Auth.Required = getEnvBool(EnvAuthRequired, true)
	r.Auth.TTL = getEnvInt(EnvTokenCacheTTL, 10)
	s, found := os.LookupEnv(EnvCatalogPath)
	if found {
		r.CatalogPath = s
	} else {
		r.CatalogPath = r.DefaultCatalogPath
	}
	s, found = os.LookupEnv(EnvPort)
	if found {
		r.Port = s
	} else {
		r.Port = "8080"
	}
	s, found = os.LookupEnv(EnvProviderName)
	if found {
		r.Provider.Name = s
	}
	s, found = os.LookupEnv(EnvProviderNamespace)
	if found {
		r.Provider.Namespace = s
	}
	s, found = os.LookupEnv(EnvProviderVerb)
	if found {
		r.Provider.Verb = s
	} else {
		r.Provider.Verb = "get"
	}
	return
}

// LoadHyperV loads HyperV-specific settings from environment variables and mounted secret.
// Should be called after Load() for HyperV provider servers.
func (r *ProviderSettings) LoadHyperV() error {
	// Required: HyperV URL
	s, found := os.LookupEnv(EnvHyperVUrl)
	if !found || s == "" {
		return fmt.Errorf("%s environment variable is required", EnvHyperVUrl)
	}
	r.HyperV.URL = s

	if s, found := os.LookupEnv(EnvHyperVUsername); found {
		r.HyperV.Username = s
	}
	if s, found := os.LookupEnv(EnvHyperVPassword); found {
		r.HyperV.Password = s
	}
	if s, found := os.LookupEnv(EnvSMBUrl); found {
		r.HyperV.SMBUrl = s
	}
	r.HyperV.RefreshInterval = DefaultRefreshInterval
	if s, found := os.LookupEnv(EnvRefreshInterval); found {
		if d, parseErr := time.ParseDuration(s); parseErr != nil {
			log.Printf("Warning: invalid %s value %q, using default %v", EnvRefreshInterval, s, DefaultRefreshInterval)
		} else {
			r.HyperV.RefreshInterval = d
		}
	}

	insecurePath := filepath.Join(SecretMountPath, "insecureSkipVerify")
	if data, err := os.ReadFile(insecurePath); err == nil {
		r.HyperV.InsecureSkipVerify, _ = strconv.ParseBool(strings.TrimSpace(string(data)))
	}

	cacertPath := filepath.Join(SecretMountPath, "cacert")
	if _, err := os.Stat(cacertPath); err == nil {
		r.HyperV.CACertPath = cacertPath
	}

	return nil
}

func getEnvBool(name string, def bool) bool {
	boolean := def
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.ParseBool(s)
		if err == nil {
			boolean = parsed
		}
	}

	return boolean
}

func getEnvInt(name string, def int) int {
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.Atoi(s)
		if err == nil {
			return parsed
		}
	}
	return def
}
