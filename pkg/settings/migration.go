package settings

import (
	"encoding/json"
	"fmt"
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
	CleanupRetries          = "CLEANUP_RETRIES"
	OvirtOsConfigMap        = "OVIRT_OS_MAP"
	VsphereOsConfigMap      = "VSPHERE_OS_MAP"
	VddkJobActiveDeadline   = "VDDK_JOB_ACTIVE_DEADLINE"
	VirtV2vExtraArgs        = "VIRT_V2V_EXTRA_ARGS"
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
	// Cleanup retries
	CleanupRetries int
	// oVirt OS config map name
	OvirtOsConfigMap string
	// vSphere OS config map name
	VsphereOsConfigMap string
	// Active deadline for VDDK validation job
	VddkJobActiveDeadline int
	// Additional arguments for virt-v2v
	VirtV2vExtraArgs string
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
	if r.CleanupRetries, err = getPositiveEnvLimit(CleanupRetries, 10); err != nil {
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

	// Set timeout to 12 hours instead of the default 2
	if r.CDIExportTokenTTL, err = getPositiveEnvLimit(CDIExportTokenTTL, 720); err != nil {
		return liberr.Wrap(err)
	}
	if r.FileSystemOverhead, err = getNonNegativeEnvLimit(FileSystemOverhead, 10); err != nil {
		return liberr.Wrap(err)
	}
	if overhead, ok := os.LookupEnv(BlockOverhead); ok {
		if quantity, err := resource.ParseQuantity(overhead); err != nil {
			return liberr.Wrap(err)
		} else if r.BlockOverhead, ok = quantity.AsInt64(); !ok {
			return fmt.Errorf("Block overhead is invalid: %s", overhead)
		}
	}
	if val, found := os.LookupEnv(OvirtOsConfigMap); found {
		r.OvirtOsConfigMap = val
	} else if Settings.Role.Has(MainRole) {
		return liberr.Wrap(fmt.Errorf("failed to find environment variable %s", OvirtOsConfigMap))
	}
	if val, found := os.LookupEnv(VsphereOsConfigMap); found {
		r.VsphereOsConfigMap = val
	} else if Settings.Role.Has(MainRole) {
		return liberr.Wrap(fmt.Errorf("failed to find environment variable %s", VsphereOsConfigMap))
	}
	if r.VddkJobActiveDeadline, err = getPositiveEnvLimit(VddkJobActiveDeadline, 300); err != nil {
		return liberr.Wrap(err)
	}
	r.VirtV2vExtraArgs = "[]"
	if val, found := os.LookupEnv(VirtV2vExtraArgs); found && len(val) > 0 {
		if encoded, jsonErr := json.Marshal(strings.Fields(val)); jsonErr == nil {
			r.VirtV2vExtraArgs = string(encoded)
		} else {
			return liberr.Wrap(err)
		}
	}

	return
}
