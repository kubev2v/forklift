package catalog

import (
	"github.com/kubev2v/forklift/pkg/lib/logging"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DownloadFilename  = "vm.ova.incomplete"
	ApplianceFilename = "vm.ova"
	StatusPending     = "Pending"
	StatusInProgress  = "InProgress"
	StatusComplete    = "Complete"
	StatusError       = "Error"
)

// ApplianceStatus defines the status of an appliance that
// the OVA provider server has been requested to download
// and store in its catalog.
type ApplianceStatus struct {
	Modified *meta.Time `json:"modified,omitempty"`
	Status   string     `json:"status"`
	URL      string     `json:"url"`
	Error    string     `json:"error,omitempty"`
	Progress int64      `json:"progress"`
	Size     int64      `json:"size"`
	staged   bool
}

// New constructs a new catalog manager.
func New(catalogPath string, sourcesPath string, scanInterval int, prune bool, concurrent int, timeout int) (m *Manager, err error) {
	m = &Manager{
		CatalogPath:            catalogPath,
		SourcePath:             sourcesPath,
		ScanInterval:           scanInterval,
		Prune:                  prune,
		MaxConcurrentDownloads: concurrent,
		DownloadTimeout:        timeout,
		statuses:               make(map[string]ApplianceStatus),
	}
	m.Log = logging.WithName("catalog")
	return
}
