package vsphere

import (
	"context"
	"errors"
	"sync"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Phases.
const (
	CopyingPaused            = "CopyingPaused"
	CreateGuestConversionPod = "CreateGuestConversionPod"
	ConvertGuest             = "ConvertGuest"
	CreateVM                 = "CreateVM"
	PostHook                 = "PostHook"
	Completed                = "Completed"
	Canceled                 = "Canceled"
)

// Steps.
const (
	DiskTransfer = "DiskTransfer"
	NotFound     = "NotFound"
)

// Package level mutex to ensure that
// multiple concurrent reconciles don't
// attempt to schedule VMs into the same
// slots.
var mutex sync.Mutex

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

// Convenience struct to package a
// VMStatus with a cost that is calculated
// from the inventory VM object.
type pendingVM struct {
	status *plan.VMStatus
	cost   int
}

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

	if hasNext {
		r.Log.Info(
			"Next scheduled VM.",
			"vm",
			vm.String())
	}

	return
}

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

	r.Log.V(1).Info(
		"Schedule built.",
		"inflight",
		r.inFlight,
		"pending",
		r.pending)

	return
}

// Build the map of the number of disks that
// are currently in flight for each host.
func (r *Scheduler) buildInFlight() (err error) {
	r.inFlight = make(map[string]int)

	// Since we modify the plan VMStatuses in memory,
	// we need to use the plan from the context rather
	// than from the list of plans that are retrieved below.
	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.HasCondition(Canceled) {
			continue
		}
		vm := &model.VM{}
		err = r.Source.Inventory.Find(vm, vmStatus.Ref)
		if err != nil {
			return
		}
		if vmStatus.Running() {
			r.inFlight[vm.Host] += r.cost(vm, vmStatus)
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

		// skip archived plans
		if p.Spec.Archived {
			continue
		}

		// skip plans that aren't being executed
		snapshot := p.Status.Migration.ActiveSnapshot()
		if !snapshot.HasCondition("Executing") {
			continue
		}

		for _, vmStatus := range p.Status.Migration.VMs {
			if !vmStatus.Running() {
				continue
			}
			vm := &model.VM{}
			err = r.Source.Inventory.Find(vm, vmStatus.Ref)
			if err != nil {
				if errors.As(err, &web.NotFoundError{}) {
					continue
				}
				if errors.As(err, &web.RefNotUniqueError{}) {
					continue
				}
				return err
			}
			r.inFlight[vm.Host] += r.cost(vm, vmStatus)
		}
	}

	return
}

// Build the map of pending VMs belonging to each host.
func (r *Scheduler) buildPending() (err error) {
	r.pending = make(map[string][]*pendingVM)

	for _, vmStatus := range r.Plan.Status.Migration.VMs {
		if vmStatus.HasCondition(Canceled) {
			continue
		}
		vm := &model.VM{}
		err = r.Source.Inventory.Find(vm, vmStatus.Ref)
		if err != nil {
			return
		}

		if !vmStatus.MarkedStarted() && !vmStatus.MarkedCompleted() {
			pending := &pendingVM{
				status: vmStatus,
				cost:   r.cost(vm, vmStatus),
			}
			r.pending[vm.Host] = append(r.pending[vm.Host], pending)
		}
	}
	return
}

func (r *Scheduler) cost(vm *model.VM, vmStatus *plan.VMStatus) int {
	useV2vForTransfer, _ := r.Plan.ShouldUseV2vForTransfer()
	if useV2vForTransfer {
		switch vmStatus.Phase {
		case CreateVM, PostHook, Completed:
			// In these phases we already have the disk transferred and are left only to create the VM
			// By setting the cost to 0 other VMs can start migrating
			return 0
		default:
			return 1
		}
	} else {
		switch vmStatus.Phase {
		case CreateVM, PostHook, Completed, CopyingPaused, ConvertGuest, CreateGuestConversionPod:
			// The warm/remote migrations this is done on already transferred disks,
			// and we can start other VM migrations at these point.
			// By setting the cost to 0 other VMs can start migrating
			return 0
		default:
			// CDI transfers the disks in parallel by different pods
			return len(vm.Disks) - r.finishedDisks(vmStatus)
		}
	}
}

// finishedDisks returns a number of the disks that have completed the disk transfer
// This can reduce the migration time as VMs with one large disks and many small disks won't halt the scheduler
func (r *Scheduler) finishedDisks(vmStatus *plan.VMStatus) int {
	var resp = 0
	for _, step := range vmStatus.Pipeline {
		if step.Name == DiskTransfer {
			for _, task := range step.Tasks {
				if task.Phase == Completed {
					resp += 1
				}
			}
		}
	}
	return resp
}

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
			// In case there is VM with more disks than the MaxInFlight MTV will migrate it, if there are no other VMs
			// being migrated at that time.
			if vms[i].cost > r.MaxInFlight && r.inFlight[host] == 0 {
				schedulable[host] = append(schedulable[host], vms[i])
			}
		}
	}

	return
}
