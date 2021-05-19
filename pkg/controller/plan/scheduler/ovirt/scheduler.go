package ovirt

import (
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"sync"
)

//
// Package level mutex to ensure that
// multiple concurrent reconciles don't
// attempt to schedule VMs into the same
// slots.
var mutex sync.Mutex

// Scheduler for migrations from oVirt.
type Scheduler struct {
	*plancontext.Context
	// Maximum number of VMs that can be
	// migrated at once per provider.
	MaxInFlight int
}

func (r *Scheduler) Next() (vm *plan.VMStatus, hasNext bool, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	hasNext = false
	return
}
