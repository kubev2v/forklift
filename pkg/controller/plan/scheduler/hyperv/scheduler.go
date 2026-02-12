package hyperv

import (
	"context"
	"sync"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

var mutex sync.Mutex

const Canceled = "Canceled"

type Scheduler struct {
	*plancontext.Context
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

	inFlight := 0
	for _, p := range planList.Items {
		if p.Spec.Provider.Source != r.Plan.Spec.Provider.Source {
			continue
		}

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

	if inFlight >= r.MaxInFlight {
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
