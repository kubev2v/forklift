package settings

import liberr "github.com/konveyor/controller/pkg/error"

//
// Environment variables.
const (
	MaxVmInFlight = "MAX_VM_INFLIGHT"
	HookDeadline  = "HOOK_DEADLINE"
	HookRetry     = "HOOK_RETRY"
)

//
// Migration settings
type Migration struct {
	// Max VMs in-flight.
	MaxInFlight int
	// Hook fail/retry limit.
	HookRetry int
	// Hook completion deadline.
	HookDeadline int
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
	r.HookRetry, err = getEnvLimit(HookRetry, 3)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}
