package settings

import (
	"os"
	"strconv"
)

const (
	EnvScanInterval           = "SCAN_INTERVAL"
	EnvSourcesPath            = "SOURCES_PATH"
	EnvCatalogPath            = "CATALOG_PATH"
	EnvPruneCatalog           = "PRUNE_CATALOG"
	EnvMaxConcurrentDownloads = "CONCURRENT_DOWNLOADS"
	EnvDownloadTimeout        = "DOWNLOAD_TIMEOUT"
	EnvPort                   = "PORT"
)

var Settings OVASettings

type OVASettings struct {
	// Path to OVA appliance directory
	CatalogPath string
	// Scan interval in seconds.
	ScanInterval int
	// Path to sources file
	SourcesPath string
	// Prune unwanted appliances
	Prune bool
	// Maximum number of concurrent downloads
	MaxConcurrentDownloads int
	// Download timeout in minutes.
	DownloadTimeout int
	// Port to serve on
	Port string
}

func (r *OVASettings) Load() (err error) {
	s, found := os.LookupEnv(EnvScanInterval)
	if found {
		n, _ := strconv.Atoi(s)
		r.ScanInterval = n
	} else {
		r.ScanInterval = 30
	}
	s, found = os.LookupEnv(EnvSourcesPath)
	if found {
		r.SourcesPath = s
	} else {
		r.SourcesPath = "/provider/sources"
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
	s, found = os.LookupEnv(EnvDownloadTimeout)
	if found {
		n, _ := strconv.Atoi(s)
		r.DownloadTimeout = n
	} else {
		r.DownloadTimeout = 30
	}
	s, found = os.LookupEnv(EnvPort)
	if found {
		r.Port = s
	} else {
		r.Port = "8080"
	}
	return
}
