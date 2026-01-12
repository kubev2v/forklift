package settings

import (
	"os"
	"strconv"
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

// getEnvBool gets a boolean from environment variable.
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

// getEnvInt gets an integer from environment variable.
func getEnvInt(name string, def int) int {
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.Atoi(s)
		if err == nil {
			return parsed
		}
	}
	return def
}
