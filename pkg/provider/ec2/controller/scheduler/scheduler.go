package scheduler

import (
	"context"
	"sync"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// mutex protects concurrent Next() calls, preventing race conditions in VM scheduling.
// Package-level mutex coordinates scheduling across all plans sharing the same source provider.
var mutex sync.Mutex

// Canceled marks VMs canceled by user. Scheduler skips these VMs.
// Allows selective VM cancellation within a plan without canceling the entire plan.
const Canceled = "Canceled"

// Scheduler manages VM migration scheduling, enforcing MaxInFlight concurrency limits.
// Controls load on EC2 infrastructure, prevents API throttling, selects next VM to migrate.
type Scheduler struct {
	*plancontext.Context // Plan context with client and provider info
	// MaxInFlight limits concurrent VMs migrating across all plans sharing source provider
	MaxInFlight int
}

// Next selects the next VM to migrate while respecting MaxInFlight limit.
// Queries all plans to count in-flight VMs, finds first unstarted non-canceled VM in order.
// Thread-safe using package-level mutex. Returns nil if limit reached or no VMs available.
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

// calcInFlight counts total VMs migrating across all plans sharing the same source provider.
// Filters plans by source, checks "Executing" condition, counts running VMs per plan.
// Enforces global MaxInFlight limit to prevent infrastructure overload and API throttling.
func (r *Scheduler) calcInFlight(planList *api.PlanList) int {
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
	return inFlight
}
