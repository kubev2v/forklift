package ovirt

import (
	"context"
	"sync"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Package level mutex to ensure that
// multiple concurrent reconciles don't
// attempt to schedule VMs into the same
// slots.
var mutex sync.Mutex

const Canceled = "Canceled"

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

	planList := &api.PlanList{}
	err = r.List(context.TODO(), planList)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if r.calcInFlight(planList) >= r.MaxInFlight {
		return
	}

	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.HasCondition(Canceled) {
			continue
		}
		if !vmStatus.MarkedStarted() && !vmStatus.MarkedCompleted() {
			vm = vmStatus
			hasNext = true
			return
		}
	}

	return
}

func (r *Scheduler) calcInFlight(planList *api.PlanList) int {
	inFlight := 0
	for _, p := range planList.Items {
		// ignore plans that aren't using the same source provider
		if p.Spec.Provider.Source != r.Plan.Spec.Provider.Source {
			continue
		}

		// skip plans that aren't being executed
		snapshot := p.Status.Migration.ActiveSnapshot()
		if !snapshot.HasCondition("Executing") {
			continue
		}

		for _, vmStatus := range p.Status.Migration.VMs {
			if vmStatus.Running() {
				inFlight++
			}
		}
	}
	return inFlight
}
