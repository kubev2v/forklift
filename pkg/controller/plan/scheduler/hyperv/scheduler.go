package hyperv

import (
	"context"
	"errors"
	"sync"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

var mutex sync.Mutex

const Canceled = "Canceled"

// Scheduler for Hyper-V migrations.
// In cluster mode, tracks in-flight migrations per host (OwnerNode) so that
// no single node is overloaded. In standalone mode, all VMs share a single
// empty-string host key, preserving the original global-counter behavior.
type Scheduler struct {
	*plancontext.Context
	MaxInFlight int
	inFlight    map[string]int
}

// Next returns the next VM to migrate, iterating in plan order to
// preserve deterministic FIFO selection across hosts.
func (r *Scheduler) Next() (vm *plan.VMStatus, hasNext bool, err error) {
	mutex.Lock()
	defer mutex.Unlock()

	if err = r.buildInFlight(); err != nil {
		return
	}

	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.HasCondition(Canceled) || vmStatus.MarkedStarted() || vmStatus.MarkedCompleted() {
			continue
		}
		host := r.hostForVM(vmStatus)
		if r.inFlight[host] >= r.MaxInFlight {
			continue
		}
		vm = vmStatus
		hasNext = true
		return
	}
	return
}

// buildInFlight counts running migrations grouped by host across all
// executing plans that share the same source provider.
func (r *Scheduler) buildInFlight() error {
	r.inFlight = make(map[string]int)

	r.countRunning(r.Plan.Status.Migration.VMs)

	planList := &api.PlanList{}
	if err := r.List(context.TODO(), planList); err != nil {
		return liberr.Wrap(err)
	}

	for i := range planList.Items {
		p := &planList.Items[i]
		if r.sharesSourceProvider(p) {
			r.countRunning(p.Status.Migration.VMs)
		}
	}
	return nil
}

// sharesSourceProvider returns true if p is another active plan migrating
// from the same Hyper-V provider. Its running VMs must be counted to avoid
// exceeding per-host concurrency limits across plans.
func (r *Scheduler) sharesSourceProvider(p *api.Plan) bool {
	if p.Name == r.Plan.Name && p.Namespace == r.Plan.Namespace {
		return false
	}
	if p.Spec.Provider.Source != r.Plan.Spec.Provider.Source {
		return false
	}
	if p.Spec.Archived {
		return false
	}
	return p.Status.Migration.ActiveSnapshot().HasCondition("Executing")
}

// countRunning increments per-host in-flight counters for running VMs.
func (r *Scheduler) countRunning(vms []*plan.VMStatus) {
	for _, vmStatus := range vms {
		if vmStatus.HasCondition(Canceled) || !vmStatus.Running() {
			continue
		}
		r.inFlight[r.hostForVM(vmStatus)]++
	}
}

// hostForVM resolves the host (OwnerNode) for a VM from inventory.
// In standalone mode or on lookup failure, returns "" which groups
// all VMs under a single key (global counter behavior).
func (r *Scheduler) hostForVM(vmStatus *plan.VMStatus) string {
	vm := &model.VM{}
	if err := r.Source.Inventory.Find(vm, vmStatus.Ref); err != nil {
		if !errors.As(err, &web.NotFoundError{}) {
			r.Log.V(1).Info(
				"Could not resolve host for VM, using global slot",
				"vm", vmStatus.String(),
				"error", err)
		}
		return ""
	}
	return vm.Host
}
