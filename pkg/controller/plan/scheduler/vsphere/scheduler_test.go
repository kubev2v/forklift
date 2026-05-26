package vsphere

import (
	"testing"

	"github.com/onsi/gomega"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	vspheremodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	webvsphere "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/settings"
)

func TestScheduler(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	hostA := "hostA"
	hostB := "hostB"
	hostC := "hostC"

	scheduler := Scheduler{MaxInFlight: 10}
	scheduler.inFlight = map[string]int{
		hostA: 6,
		hostB: 10,
		hostC: 0,
	}
	scheduler.pending = map[string][]*pendingVM{
		// Only VMs that fit the available capacity
		// can be scheduled. Host A already has 6 slots occupied,
		// so only the VM with cost 4 can be scheduled.
		hostA: {
			{
				cost: 6,
			},
			{
				cost: 5,
			},
			{
				cost: 4,
			},
			{
				cost: 7,
			},
		},

		// host B has reached capacity, so we
		// can't schedule any migrations from it.
		hostB: {
			{
				cost: 10,
			},
			{
				cost: 0,
			},
			{
				cost: 1,
			},
		},

		// host C is unoccupied, so any of its
		// vms with a cost of 10 or less could
		// be started
		hostC: {
			{
				cost: 11,
			},
			{
				cost: 1,
			},
			{
				cost: 2,
			},
			{
				cost: 3,
			},
			{
				cost: 10,
			},
		},
	}

	// no VMs from host B could be scheduled, so we shouldn't see
	// an entry for host B in the schedule map.
	expectedSchedule := map[string][]*pendingVM{
		hostA: {
			{
				cost: 4,
			},
		},
		hostC: {
			{
				cost: 11,
			},
			{
				cost: 1,
			},
			{
				cost: 2,
			},
			{
				cost: 3,
			},
			{
				cost: 10,
			},
		},
	}
	g.Expect(scheduler.schedulable()).To(gomega.Equal(expectedSchedule))
}

// v2vPlan returns a Plan that makes ShouldUseV2vForTransfer() return true:
// cold VSphere→host migration with MigrateSharedDisks=true.
func v2vPlan() *api.Plan {
	vsphereType := api.VSphere
	ocpType := api.OpenShift

	sourceProvider := &api.Provider{}
	sourceProvider.Spec.Type = &vsphereType

	destProvider := &api.Provider{}
	destProvider.Spec.Type = &ocpType
	destProvider.Spec.URL = "" // empty URL → IsHost()

	p := &api.Plan{}
	p.Provider.Source = sourceProvider
	p.Provider.Destination = destProvider
	p.Spec.MigrateSharedDisks = true
	// Referenced.Map.Storage left nil → skips HasNetAppShiftDestination check
	return p
}

func vmWithDisks(n int) *webvsphere.VM {
	vm := &webvsphere.VM{}
	for range n {
		vm.Disks = append(vm.Disks, vspheremodel.Disk{})
	}
	return vm
}

func schedulerWithPlan(p *api.Plan) *Scheduler {
	return &Scheduler{
		Context: &plancontext.Context{
			Plan: p,
		},
	}
}

func TestCost_V2vPlan_CopyOffloadDisabled(t *testing.T) {
	settings.Settings.CopyOffload = false
	defer func() { settings.Settings.CopyOffload = false }()

	s := schedulerWithPlan(v2vPlan())
	vm := vmWithDisks(5)
	vmStatus := &plan.VMStatus{Phase: DiskTransfer}
	cost := s.cost(vm, vmStatus)
	if cost != 1 {
		t.Errorf("expected cost=1 for v2v plan without CopyOffload, got %d", cost)
	}
}

func TestCost_V2vPlan_CopyOffloadEnabled(t *testing.T) {
	settings.Settings.CopyOffload = true
	defer func() { settings.Settings.CopyOffload = false }()

	s := schedulerWithPlan(v2vPlan())
	vm := vmWithDisks(5)
	vmStatus := &plan.VMStatus{Phase: DiskTransfer}
	cost := s.cost(vm, vmStatus)
	// With CopyOffload on, a v2v plan must count by disk, not return 1.
	if cost != 5 {
		t.Errorf("expected cost=5 (disk count) for v2v+CopyOffload plan, got %d", cost)
	}
}

func TestCost_CopyOffload_ZeroCostPhases(t *testing.T) {
	settings.Settings.CopyOffload = true
	defer func() { settings.Settings.CopyOffload = false }()

	s := schedulerWithPlan(v2vPlan())
	vm := vmWithDisks(5)
	zeroCostPhases := []string{CreateVM, PostHook, Completed, CopyingPaused, ConvertGuest, CreateGuestConversionPod}
	for _, phase := range zeroCostPhases {
		vmStatus := &plan.VMStatus{Phase: phase}
		if cost := s.cost(vm, vmStatus); cost != 0 {
			t.Errorf("phase %q: expected cost=0, got %d", phase, cost)
		}
	}
}
