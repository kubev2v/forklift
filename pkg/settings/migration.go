package settings

import (
	liberr "github.com/konveyor/controller/pkg/error"
	"os"
)

//
// Environment variables.
const (
	MaxVmInFlight   = "MAX_VM_INFLIGHT"
	HookRetry       = "HOOK_RETRY"
	ImporterRetry   = "IMPORTER_RETRY"
	VirtV2vImage    = "VIRT_V2V_IMAGE"
	PrecopyInterval = "PRECOPY_INTERVAL"
)

//
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
	// Virt-v2v image for guest conversion
	VirtV2vImage string
}

//
// Load settings.
func (r *Migration) Load() (err error) {
	r.MaxInFlight, err = getEnvLimit(MaxVmInFlight, 20)
	if err != nil {
		err = liberr.Wrap(err)
	}
	r.HookRetry, err = getEnvLimit(HookRetry, 3)
	if err != nil {
		err = liberr.Wrap(err)
	}
	r.ImporterRetry, err = getEnvLimit(ImporterRetry, 3)
	if err != nil {
		err = liberr.Wrap(err)
	}
	r.PrecopyInterval, err = getEnvLimit(PrecopyInterval, 60)
	if err != nil {
		err = liberr.Wrap(err)
	}
	r.VirtV2vImage = os.Getenv(VirtV2vImage)

	return
}
