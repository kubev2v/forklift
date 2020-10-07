package settings

import liberr "github.com/konveyor/controller/pkg/error"

//
// Environment variables.
const (
	MaxVmInFlight = "MAX_VM_INFLIGHT"
)

//
// Migration settings
type Migration struct {
	// Max VMs in-flight.
	MaxInFlight int
}

//
// Load settings.
func (r *Migration) Load() (err error) {
	r.MaxInFlight, err = getEnvLimit(MaxVmInFlight, 20)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}
