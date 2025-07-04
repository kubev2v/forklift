package settings

import (
	"os"
	"strconv"
)

const (
	EnvScanInterval           = "SCAN_INTERVAL"
	EnvConfigPath             = "CONFIG_PATH"
	EnvCatalogPath            = "CATALOG_PATH"
	EnvPruneCatalog           = "PRUNE_CATALOG"
	EnvMaxConcurrentDownloads = "CONCURRENT_DOWNLOADS"
)

var Settings OVASettings

type OVASettings struct {
	// Path to OVA appliance directory
	CatalogPath string
	// Scan interval in seconds.
	ScanInterval int
	// Path to config file
	ConfigPath string
	// Prune unwanted appliances
	Prune bool
	// Maximum number of concurrent downloads
	MaxConcurrentDownloads int
}

func (r *OVASettings) Load() (err error) {
	s, found := os.LookupEnv(EnvScanInterval)
	if found {
		n, _ := strconv.Atoi(s)
		r.ScanInterval = n
	} else {
		r.ScanInterval = 30
	}
	s, found = os.LookupEnv(EnvConfigPath)
	if found {
		r.ConfigPath = s
	} else {
		r.ConfigPath = "/provider/settings.yaml"
	}
	s, found = os.LookupEnv(EnvCatalogPath)
	if found {
		r.CatalogPath = s
	} else {
		r.CatalogPath = "/ova"
	}
	s, found = os.LookupEnv(EnvPruneCatalog)
	if found {
		r.Prune, _ = strconv.ParseBool(s)
	}
	s, found = os.LookupEnv(EnvMaxConcurrentDownloads)
	if found {
		n, _ := strconv.Atoi(s)
		r.MaxConcurrentDownloads = n
	} else {
		r.MaxConcurrentDownloads = 3
	}
	return
}
