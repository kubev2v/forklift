package vsphere

import (
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
)

//
// Scheduler for migrations from ESX hosts.
type Scheduler struct {
	*plancontext.Context
	MaxInFlight int
	inflight    map[string]int
	pending     map[string][]*pendingVM
}

type pendingVM struct {
	status *plan.VMStatus
	cost   int
}

//
// Return the next VM to migrate.
func (r *Scheduler) Next() (vm *plan.VMStatus, err error) {
	err = r.buildSchedule()
	if err != nil {
		return
	}
	for _, vms := range r.schedulable() {
		if len(vms) > 0 {
			vm = vms[0].status
		}
	}
	return
}

//
// Determine how much host capcity is occupied
// by running migrations across all plans for
// the same provider, and determine which
// VMs are still waiting to be started.
func (r *Scheduler) buildSchedule() error {
	r.inflight = make(map[string]int)
	r.pending = make(map[string][]*pendingVM)

	// Since we modify the plan VMStatuses in memory,
	// we need to use the plan from the context rather
	// than from the list of plans that are retrieved below.
	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		obj, err := r.Source.Inventory.VM(&vmStatus.Ref)
		if err != nil {
			return err
		}
		vm := obj.(*vsphere.VM)
		if vmStatus.Running() {
			r.inflight[vm.Host.ID] += len(vm.Disks)
		} else if !vmStatus.MarkedCompleted() {
			pending := &pendingVM{
				status: vmStatus,
				cost:   len(vm.Disks),
			}
			r.pending[vm.Host.ID] = append(r.pending[vm.Host.ID], pending)
		}
	}

	planList := &api.PlanList{}
	err := r.List(context.TODO(), planList)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, p := range planList.Items {
		// skip this plan, it's already done.
		if p.Name == r.Plan.Name && p.Namespace == r.Plan.Namespace {
			continue
		}
		// ignore plans that aren't using the same source provider
		if p.Spec.Provider.Source != r.Plan.Spec.Provider.Source {
			continue
		}

		for _, vmStatus := range p.Status.Migration.VMs {
			obj, err := r.Source.Inventory.VM(&vmStatus.Ref)
			if err != nil {
				return err
			}
			vm := obj.(*vsphere.VM)
			if vmStatus.Running() {
				r.inflight[vm.Host.ID] += len(vm.Disks)
			}
		}
	}

	return nil
}

//
// Return a map of all the VMs that could be scheduled
// based on the available host capacities.
func (r *Scheduler) schedulable() map[string][]*pendingVM {
	schedulable := make(map[string][]*pendingVM)
	for host, vms := range r.pending {
		if r.inflight[host] >= r.MaxInFlight {
			continue
		}
		for i := range vms {
			if vms[i].cost+r.inflight[host] <= r.MaxInFlight {
				schedulable[host] = append(schedulable[host], vms[i])
			}
		}
	}

	return schedulable
}
