package settings

import (
	"fmt"
	"os"
	"strings"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

// Environment variables.
const (
	MaxVmInFlight           = "MAX_VM_INFLIGHT"
	HookRetry               = "HOOK_RETRY"
	ImporterRetry           = "IMPORTER_RETRY"
	VirtV2vImage            = "VIRT_V2V_IMAGE"
	PrecopyInterval         = "PRECOPY_INTERVAL"
	VirtV2vDontRequestKVM   = "VIRT_V2V_DONT_REQUEST_KVM"
	SnapshotRemovalTimeout  = "SNAPSHOT_REMOVAL_TIMEOUT"
	SnapshotStatusCheckRate = "SNAPSHOT_STATUS_CHECK_RATE"
	CDIExportTokenTTL       = "CDI_EXPORT_TOKEN_TTL"
	FileSystemOverhead      = "FILESYSTEM_OVERHEAD"
	BlockOverhead           = "BLOCK_OVERHEAD"
)

// Migration settings
type Migration struct {
	// Max VMs in-flight.
	MaxInFlight int
	// Hook fail/retry limit.
	HookRetry int
	// Importer pod retry limit.
	ImporterRetry int
	// Warm migration precopy interval in minutes
	PrecopyInterval int
	// Snapshot removal timeout in minutes
	SnapshotRemovalTimeout int
	// Snapshot status check rate in seconds
	SnapshotStatusCheckRate int
	// Virt-v2v images for guest conversion
	VirtV2vImageCold string
	VirtV2vImageWarm string
	// Virt-v2v require KVM flags for guest conversion
	VirtV2vDontRequestKVM bool
	// OCP Export token TTL minutes
	CDIExportTokenTTL int
	// FileSystem overhead in percantage
	FileSystemOverhead int
	// Block fixed overhead size
	BlockOverhead int
}

// Load settings.
func (r *Migration) Load() (err error) {
	if r.MaxInFlight, err = getPositiveEnvLimit(MaxVmInFlight, 20); err != nil {
		return liberr.Wrap(err)
	}
	if r.HookRetry, err = getPositiveEnvLimit(HookRetry, 3); err != nil {
		return liberr.Wrap(err)
	}
	if r.ImporterRetry, err = getPositiveEnvLimit(ImporterRetry, 3); err != nil {
		return liberr.Wrap(err)
	}
	if r.PrecopyInterval, err = getPositiveEnvLimit(PrecopyInterval, 60); err != nil {
		return liberr.Wrap(err)
	}
	if r.SnapshotRemovalTimeout, err = getPositiveEnvLimit(SnapshotRemovalTimeout, 120); err != nil {
		return liberr.Wrap(err)
	}
	if r.SnapshotStatusCheckRate, err = getPositiveEnvLimit(SnapshotStatusCheckRate, 10); err != nil {
		return liberr.Wrap(err)
	}
	if virtV2vImage, ok := os.LookupEnv(VirtV2vImage); ok {
		if cold, warm, found := strings.Cut(virtV2vImage, "|"); found {
			r.VirtV2vImageCold = cold
			r.VirtV2vImageWarm = warm
		} else {
			r.VirtV2vImageCold = virtV2vImage
			r.VirtV2vImageWarm = virtV2vImage
		}
	} else if Settings.Role.Has(MainRole) {
		return liberr.Wrap(fmt.Errorf("failed to find environment variable %s", VirtV2vImage))
	}
	r.VirtV2vDontRequestKVM = getEnvBool(VirtV2vDontRequestKVM, false)
	if r.CDIExportTokenTTL, err = getPositiveEnvLimit(CDIExportTokenTTL, 0); err != nil {
		return liberr.Wrap(err)
	}
	if r.FileSystemOverhead, err = getNonNegativeEnvLimit(FileSystemOverhead, 10); err != nil {
		return liberr.Wrap(err)
	}
	if r.BlockOverhead, err = getNonNegativeEnvLimit(BlockOverhead, 0); err != nil {
		return liberr.Wrap(err)
	}

	return
}
