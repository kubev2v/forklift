package settings

import (
	"os"
	"strings"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"k8s.io/apimachinery/pkg/api/resource"
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

// Default virt-v2v image.
const (
	DefaultVirtV2vImage = "quay.io/kubev2v/forklift-virt-v2v:latest"
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
	BlockOverhead int64
}

// Load settings.
func (r *Migration) Load() (err error) {
	r.MaxInFlight, err = getPositiveEnvLimit(MaxVmInFlight, 20)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.HookRetry, err = getPositiveEnvLimit(HookRetry, 3)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.ImporterRetry, err = getPositiveEnvLimit(ImporterRetry, 3)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.PrecopyInterval, err = getPositiveEnvLimit(PrecopyInterval, 60)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.SnapshotRemovalTimeout, err = getPositiveEnvLimit(SnapshotRemovalTimeout, 120)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.SnapshotStatusCheckRate, err = getPositiveEnvLimit(SnapshotStatusCheckRate, 10)
	if err != nil {
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
	} else {
		r.VirtV2vImageCold = DefaultVirtV2vImage
		r.VirtV2vImageWarm = DefaultVirtV2vImage
	}
	r.VirtV2vDontRequestKVM = getEnvBool(VirtV2vDontRequestKVM, false)
	r.CDIExportTokenTTL, err = getPositiveEnvLimit(CDIExportTokenTTL, 0)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.FileSystemOverhead, err = getNonNegativeEnvLimit(FileSystemOverhead, 10)
	if err != nil {
		return liberr.Wrap(err)
	}
	if overhead, ok := os.LookupEnv(BlockOverhead); ok {
		if quantity, err := resource.ParseQuantity(overhead); err != nil {
			return liberr.Wrap(err)
		} else if r.BlockOverhead, ok = quantity.AsInt64(); !ok {
			return fmt.Errorf("Block overhead is invalid: %s", overhead)
		}
	}

	return
}
