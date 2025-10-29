package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Environment variables.
const (
	MaxVmInFlight                  = "MAX_VM_INFLIGHT"
	HookRetry                      = "HOOK_RETRY"
	ImporterRetry                  = "IMPORTER_RETRY"
	VirtV2vImage                   = "VIRT_V2V_IMAGE"
	vddkImage                      = "VDDK_IMAGE"
	PrecopyInterval                = "PRECOPY_INTERVAL"
	VirtV2vDontRequestKVM          = "VIRT_V2V_DONT_REQUEST_KVM"
	SnapshotRemovalTimeout         = "SNAPSHOT_REMOVAL_TIMEOUT"
	SnapshotStatusCheckRate        = "SNAPSHOT_STATUS_CHECK_RATE"
	CDIExportTokenTTL              = "CDI_EXPORT_TOKEN_TTL"
	FileSystemOverhead             = "FILESYSTEM_OVERHEAD"
	BlockOverhead                  = "BLOCK_OVERHEAD"
	CleanupRetries                 = "CLEANUP_RETRIES"
	DvStatusCheckRetries           = "DV_STATUS_CHECK_RETRIES"
	SnapshotRemovalCheckRetries    = "SNAPSHOT_REMOVAL_CHECK_RETRIES"
	OvirtOsConfigMap               = "OVIRT_OS_MAP"
	VsphereOsConfigMap             = "VSPHERE_OS_MAP"
	VirtCustomizeConfigMap         = "VIRT_CUSTOMIZE_MAP"
	VddkJobActiveDeadline          = "VDDK_JOB_ACTIVE_DEADLINE"
	VirtV2vExtraArgs               = "VIRT_V2V_EXTRA_ARGS"
	VirtV2vExtraConfConfigMap      = "VIRT_V2V_EXTRA_CONF_CONFIG_MAP"
	VirtV2vContainerLimitsCpu      = "VIRT_V2V_CONTAINER_LIMITS_CPU"
	VirtV2vContainerLimitsMemory   = "VIRT_V2V_CONTAINER_LIMITS_MEMORY"
	VirtV2vContainerRequestsCpu    = "VIRT_V2V_CONTAINER_REQUESTS_CPU"
	VirtV2vContainerRequestsMemory = "VIRT_V2V_CONTAINER_REQUESTS_MEMORY"
	HooksContainerLimitsCpu        = "HOOKS_CONTAINER_LIMITS_CPU"
	HooksContainerLimitsMemory     = "HOOKS_CONTAINER_LIMITS_MEMORY"
	HooksContainerRequestsCpu      = "HOOKS_CONTAINER_REQUESTS_CPU"
	HooksContainerRequestsMemory   = "HOOKS_CONTAINER_REQUESTS_MEMORY"
	OvaContainerLimitsCpu          = "OVA_CONTAINER_LIMITS_CPU"
	OvaContainerLimitsMemory       = "OVA_CONTAINER_LIMITS_MEMORY"
	OvaContainerRequestsCpu        = "OVA_CONTAINER_REQUESTS_CPU"
	OvaContainerRequestsMemory     = "OVA_CONTAINER_REQUESTS_MEMORY"
	TlsConnectionTimeout           = "TLS_CONNECTION_TIMEOUT"
	MaxConcurrentReconciles        = "MAX_CONCURRENT_RECONCILES"
	MaxParentBackingRetries        = "MAX_PARENT_BACKING_RETRIES"
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
	// Virt-v2v image for guest conversion
	VirtV2vImage string
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
	// DvStatusCheckRetries retries
	DvStatusCheckRetries int
	// SnapshotRemovalCheckRetries retries
	SnapshotRemovalCheckRetries int
	// oVirt OS config map name
	OvirtOsConfigMap string
	// vSphere OS config map name
	VsphereOsConfigMap string
	// vSphere OS config map name
	VirtCustomizeConfigMap string
	// Active deadline for VDDK validation job
	VddkJobActiveDeadline int
	// Additional arguments for virt-v2v
	VirtV2vExtraArgs string
	// Additional configuration for virt-v2v
	VirtV2vExtraConfConfigMap      string
	VirtV2vContainerLimitsCpu      string
	VirtV2vContainerLimitsMemory   string
	VirtV2vContainerRequestsCpu    string
	VirtV2vContainerRequestsMemory string
	HooksContainerLimitsCpu        string
	HooksContainerLimitsMemory     string
	HooksContainerRequestsCpu      string
	HooksContainerRequestsMemory   string
	OvaContainerLimitsCpu          string
	OvaContainerLimitsMemory       string
	OvaContainerRequestsCpu        string
	OvaContainerRequestsMemory     string
	// VDDK image for guest conversion
	VddkImage string
	// TlsConnectionTimeout is the timeout for TLS connections in seconds
	TlsConnectionTimeout int
	// MaxConcurrentReconciles is the limit of how many reconciles can run at once
	MaxConcurrentReconciles int
	// MaxParentBackingRetries is the limit of how many retries can happen while getting parent backing of a disk
	MaxParentBackingRetries int
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
	if virtCustomizeConfigMap, ok := os.LookupEnv(VirtCustomizeConfigMap); ok {
		r.VirtCustomizeConfigMap = virtCustomizeConfigMap
	} else if Settings.Role.Has(MainRole) {
		return liberr.Wrap(fmt.Errorf("failed to find environment variable %s", VirtCustomizeConfigMap))
	}
	if r.CleanupRetries, err = getPositiveEnvLimit(CleanupRetries, 10); err != nil {
		return liberr.Wrap(err)
	}
	if r.DvStatusCheckRetries, err = getPositiveEnvLimit(DvStatusCheckRetries, 10); err != nil {
		return liberr.Wrap(err)
	}
	if r.SnapshotRemovalCheckRetries, err = getPositiveEnvLimit(SnapshotRemovalCheckRetries, 20); err != nil {
		return liberr.Wrap(err)
	}
	if virtV2vImage, ok := os.LookupEnv(VirtV2vImage); ok {
		r.VirtV2vImage = virtV2vImage
	} else if Settings.Role.Has(MainRole) {
		return liberr.Wrap(fmt.Errorf("failed to find environment variable %s", VirtV2vImage))
	}
	r.VirtV2vDontRequestKVM = getEnvBool(VirtV2vDontRequestKVM, false)

	// VDDK image for guest conversion
	if vddkImage, ok := os.LookupEnv(vddkImage); ok {
		r.VddkImage = vddkImage
	}

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
	if r.TlsConnectionTimeout, err = getPositiveEnvLimit(TlsConnectionTimeout, 5); err != nil {
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
	if val, found := os.LookupEnv(VirtV2vExtraConfConfigMap); found {
		r.VirtV2vExtraConfConfigMap = val
	}
	// Containers configurations
	if val, found := os.LookupEnv(VirtV2vContainerLimitsCpu); found {
		r.VirtV2vContainerLimitsCpu = val
	} else {
		r.VirtV2vContainerLimitsCpu = "4000m"
	}
	if val, found := os.LookupEnv(VirtV2vContainerLimitsMemory); found {
		r.VirtV2vContainerLimitsMemory = val
	} else {
		r.VirtV2vContainerLimitsMemory = "8Gi"
	}
	if val, found := os.LookupEnv(VirtV2vContainerRequestsCpu); found {
		r.VirtV2vContainerRequestsCpu = val
	} else {
		r.VirtV2vContainerRequestsCpu = "1000m"
	}
	if val, found := os.LookupEnv(VirtV2vContainerRequestsMemory); found {
		r.VirtV2vContainerRequestsMemory = val
	} else {
		r.VirtV2vContainerRequestsMemory = "1Gi"
	}
	if val, found := os.LookupEnv(HooksContainerLimitsCpu); found {
		r.HooksContainerLimitsCpu = val
	} else {
		r.HooksContainerLimitsCpu = "1000m"
	}
	if val, found := os.LookupEnv(HooksContainerLimitsMemory); found {
		r.HooksContainerLimitsMemory = val
	} else {
		r.HooksContainerLimitsMemory = "1Gi"
	}
	if val, found := os.LookupEnv(HooksContainerRequestsCpu); found {
		r.HooksContainerRequestsCpu = val
	} else {
		r.HooksContainerRequestsCpu = "100m"
	}
	if val, found := os.LookupEnv(HooksContainerRequestsMemory); found {
		r.HooksContainerRequestsMemory = val
	} else {
		r.HooksContainerRequestsMemory = "150Mi"
	}
	if val, found := os.LookupEnv(OvaContainerLimitsCpu); found {
		r.OvaContainerLimitsCpu = val
	} else {
		r.OvaContainerLimitsCpu = "1000m"
	}
	if val, found := os.LookupEnv(OvaContainerLimitsMemory); found {
		r.OvaContainerLimitsMemory = val
	} else {
		r.OvaContainerLimitsMemory = "1Gi"
	}
	if val, found := os.LookupEnv(OvaContainerRequestsCpu); found {
		r.OvaContainerRequestsCpu = val
	} else {
		r.OvaContainerRequestsCpu = "100m"
	}
	if val, found := os.LookupEnv(OvaContainerRequestsMemory); found {
		r.OvaContainerRequestsMemory = val
	} else {
		r.OvaContainerRequestsMemory = "512Mi"
	}
	r.MaxConcurrentReconciles, err = getPositiveEnvLimit(MaxConcurrentReconciles, 10)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.MaxParentBackingRetries, err = getPositiveEnvLimit(MaxParentBackingRetries, 10)
	if err != nil {
		return liberr.Wrap(err)
	}
	return
}
