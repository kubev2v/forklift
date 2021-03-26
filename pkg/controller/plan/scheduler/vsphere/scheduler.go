package vsphere

import (
	"context"
	"sync"

	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
)

//
// Package level mutex to ensure that
// multiple concurrent reconciles don't
// attempt to schedule VMs into the same
// slots.
var mutex sync.Mutex

//
// Scheduler for migrations from ESX hosts.
type Scheduler struct {
	*plancontext.Context
	// Maximum number of disks per host that can be
	// migrated at once.
	MaxInFlight int
	// Mapping of hosts by ID to the number of disks
	// on each host that are currently being migrated.
	inFlight map[string]int
	// Mapping of hosts by ID to lists of VMs
	// that are waiting to be migrated.
	pending map[string][]*pendingVM
}

//
// Convenience struct to package a
// VMStatus with a cost that is calculated
// from the inventory VM object.
type pendingVM struct {
	status *plan.VMStatus
	cost   int
}

//
// Return the next VM to migrate.
func (r *Scheduler) Next() (vm *plan.VMStatus, hasNext bool, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	err = r.buildSchedule()
	if err != nil {
		return
	}
	for _, vms := range r.schedulable() {
		if len(vms) > 0 {
			vm = vms[0].status
			hasNext = true
		}
	}
	return
}

//
// Determine how much host capacity is occupied
// by running migrations across all plans for
// the same provider, and determine which
// VMs are still waiting to be started.
func (r *Scheduler) buildSchedule() (err error) {
	err = r.buildInFlight()
	if err != nil {
		return
	}

	err = r.buildPending()
	if err != nil {
		return
	}

	return
}

//
// Build the map of the number of disks that
// are currently in flight for each host.
func (r *Scheduler) buildInFlight() (err error) {
	r.inFlight = make(map[string]int)

	// Since we modify the plan VMStatuses in memory,
	// we need to use the plan from the context rather
	// than from the list of plans that are retrieved below.
	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		vm := &model.VM{}
		err = r.Source.Inventory.Find(vm, vmStatus.Ref)
		if err != nil {
			return
		}
		if vmStatus.Running() {
			r.inFlight[vm.Host.ID] += len(vm.Disks)
		}
	}

	planList := &api.PlanList{}
	err = r.List(context.TODO(), planList)
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
			vm := &model.VM{}
			err = r.Source.Inventory.Find(vm, vmStatus.Ref)
			if err != nil {
				return err
			}
			if vmStatus.Running() {
				r.inFlight[vm.Host.ID] += len(vm.Disks)
			}
		}
	}

	return
}

//
// Build the map of pending VMs belonging to each host.
func (r *Scheduler) buildPending() (err error) {
	r.pending = make(map[string][]*pendingVM)

	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		vm := &model.VM{}
		err = r.Source.Inventory.Find(vm, vmStatus.Ref)
		if err != nil {
			return
		}

		if !vmStatus.MarkedStarted() && !vmStatus.MarkedCompleted() {
			pending := &pendingVM{
				status: vmStatus,
				cost:   len(vm.Disks),
			}
			r.pending[vm.Host.ID] = append(r.pending[vm.Host.ID], pending)
		}
	}
	return
}

//
// Return a map of all the VMs that could be scheduled
// based on the available host capacities.
func (r *Scheduler) schedulable() (schedulable map[string][]*pendingVM) {
	schedulable = make(map[string][]*pendingVM)
	for host, vms := range r.pending {
		if r.inFlight[host] >= r.MaxInFlight {
			continue
		}
		for i := range vms {
			if vms[i].cost+r.inFlight[host] <= r.MaxInFlight {
				schedulable[host] = append(schedulable[host], vms[i])
			}
		}
	}

	return
}
