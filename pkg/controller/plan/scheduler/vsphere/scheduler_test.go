package vsphere

import (
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	vspheremodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func newVMStatus(id string) *plan.VMStatus {
	s := &plan.VMStatus{}
	s.ID = id
	return s
}

// buildPendingFiltered mirrors the logic from Scheduler.buildPending()
// without requiring a real inventory or plan context.
func buildPendingFiltered(all []testVM) map[string][]*pendingVM {
	hasActiveCreators := false
	for _, v := range all {
		if v.hasShared && v.migratesShared && !v.status.MarkedCompleted() {
			hasActiveCreators = true
			break
		}
	}

	pending := make(map[string][]*pendingVM)
	for _, v := range all {
		if v.status.MarkedStarted() || v.status.MarkedCompleted() {
			continue
		}
		if hasActiveCreators && v.hasShared && !v.migratesShared {
			continue
		}
		pending[v.host] = append(pending[v.host], &pendingVM{status: v.status, cost: 1})
	}
	return pending
}

type testVM struct {
	status         *plan.VMStatus
	host           string
	hasShared      bool
	migratesShared bool
}

func TestSharedDiskPriority(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("creator blocks consumer but not non-shared", func(_ *testing.T) {
		pending := buildPendingFiltered([]testVM{
			{status: newVMStatus("consumer"), host: "h1", hasShared: true, migratesShared: false},
			{status: newVMStatus("creator"), host: "h1", hasShared: true, migratesShared: true},
			{status: newVMStatus("normal"), host: "h1", hasShared: false},
		})
		g.Expect(pending["h1"]).To(gomega.HaveLen(2))
		g.Expect(pending["h1"][0].status.ID).To(gomega.Equal("creator"))
		g.Expect(pending["h1"][1].status.ID).To(gomega.Equal("normal"))
	})

	t.Run("completed creator unblocks consumer", func(_ *testing.T) {
		done := newVMStatus("creator-done")
		done.MarkStarted()
		done.MarkCompleted()

		pending := buildPendingFiltered([]testVM{
			{status: done, host: "h1", hasShared: true, migratesShared: true},
			{status: newVMStatus("consumer"), host: "h1", hasShared: true, migratesShared: false},
			{status: newVMStatus("normal"), host: "h1", hasShared: false},
		})
		g.Expect(pending["h1"]).To(gomega.HaveLen(2))
		g.Expect(pending["h1"][0].status.ID).To(gomega.Equal("consumer"))
		g.Expect(pending["h1"][1].status.ID).To(gomega.Equal("normal"))
	})

	t.Run("no shared-disk VMs allows all", func(_ *testing.T) {
		pending := buildPendingFiltered([]testVM{
			{status: newVMStatus("a"), host: "h1", hasShared: false},
			{status: newVMStatus("b"), host: "h1", hasShared: false},
		})
		g.Expect(pending["h1"]).To(gomega.HaveLen(2))
	})

	t.Run("running creator still blocks consumer", func(_ *testing.T) {
		running := newVMStatus("creator-running")
		running.MarkStarted()

		pending := buildPendingFiltered([]testVM{
			{status: running, host: "h1", hasShared: true, migratesShared: true},
			{status: newVMStatus("consumer"), host: "h1", hasShared: true, migratesShared: false},
		})
		g.Expect(pending["h1"]).To(gomega.BeEmpty())
	})

	t.Run("cross-host creators both scheduled alongside non-shared", func(_ *testing.T) {
		pending := buildPendingFiltered([]testVM{
			{status: newVMStatus("creator-h1"), host: "h1", hasShared: true, migratesShared: true},
			{status: newVMStatus("creator-h2"), host: "h2", hasShared: true, migratesShared: true},
			{status: newVMStatus("normal"), host: "h1", hasShared: false},
		})
		g.Expect(pending["h1"]).To(gomega.HaveLen(2))
		g.Expect(pending["h1"][0].status.ID).To(gomega.Equal("creator-h1"))
		g.Expect(pending["h1"][1].status.ID).To(gomega.Equal("normal"))
		g.Expect(pending["h2"]).To(gomega.HaveLen(1))
		g.Expect(pending["h2"][0].status.ID).To(gomega.Equal("creator-h2"))
	})

	t.Run("user scenario: consumer listed first but creator runs first", func(_ *testing.T) {
		pending := buildPendingFiltered([]testVM{
			{status: newVMStatus("vm-3025"), host: "h1", hasShared: true, migratesShared: false},
			{status: newVMStatus("vm-2998"), host: "h1", hasShared: true, migratesShared: true},
		})
		g.Expect(pending["h1"]).To(gomega.HaveLen(1))
		g.Expect(pending["h1"][0].status.ID).To(gomega.Equal("vm-2998"))
	})
}

type stubInventory struct {
	base.Client
	vms map[string]model.VM
}

func (s *stubInventory) Find(resource interface{}, r ref.Ref) error {
	switch res := resource.(type) {
	case *model.VM:
		vm, ok := s.vms[r.ID]
		if !ok {
			return base.NotFoundError{Ref: r}
		}
		*res = vm
	}
	return nil
}

func inventoryVM(id, host string, sharedDisks ...bool) model.VM {
	vm := model.VM{}
	vm.ID = id
	vm.Name = id
	vm.Host = host
	if len(sharedDisks) == 0 {
		vm.Disks = []vspheremodel.Disk{
			{File: id + "-disk0"},
			{File: id + "-disk1"},
		}
		return vm
	}
	for i, shared := range sharedDisks {
		vm.Disks = append(vm.Disks, vspheremodel.Disk{
			File:   fmt.Sprintf("%s-disk%d", id, i),
			Shared: shared,
		})
	}
	return vm
}

func newTestScheduler(vms []*plan.VMStatus, inv *stubInventory) *Scheduler {
	scheme := runtime.NewScheme()
	_ = api.SchemeBuilder.AddToScheme(scheme)
	pt := api.VSphere
	return &Scheduler{
		Context: &plancontext.Context{
			Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
			Plan: &api.Plan{
				Spec: api.PlanSpec{
					Provider: provider.Pair{
						Source: core.ObjectReference{Name: "src", Namespace: "ns"},
					},
				},
				Status: api.PlanStatus{
					Migration: plan.MigrationStatus{VMs: vms},
				},
			},
			Source: plancontext.Source{
				Provider:  &api.Provider{Spec: api.ProviderSpec{Type: &pt}},
				Inventory: inv,
			},
			Log: logging.WithName("test"),
		},
		MaxInFlight: 10,
	}
}

func vmRef(id string) ref.Ref {
	return ref.Ref{ID: id, Name: id}
}

func TestMarkNotFound_queuedVM(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	vmStatus := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("missing-queued")},
		Phase: api.PhaseStarted,
	}
	scheduler := newTestScheduler([]*plan.VMStatus{vmStatus}, &stubInventory{})

	scheduler.markNotFound(vmStatus)

	g.Expect(vmStatus.MarkedCompleted()).To(gomega.BeTrue())
	g.Expect(vmStatus.Phase).To(gomega.Equal(api.PhaseCompleted))
	g.Expect(vmStatus.HasCondition(api.ConditionCanceled)).To(gomega.BeTrue())
	g.Expect(vmStatus.HasCondition(api.ConditionFailed)).To(gomega.BeFalse())
	g.Expect(vmStatus.Error).To(gomega.BeNil())
	condition := vmStatus.FindCondition(api.ConditionCanceled)
	g.Expect(condition.Reason).To(gomega.Equal(NotFound))
}

func TestMarkNotFound_runningVM(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	vmStatus := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("missing-running")},
		Phase: api.PhaseStarted,
		Pipeline: []*plan.Step{
			{Task: plan.Task{Name: DiskTransfer, Phase: api.StepRunning}},
		},
	}
	vmStatus.MarkStarted()
	scheduler := newTestScheduler([]*plan.VMStatus{vmStatus}, &stubInventory{})

	scheduler.markNotFound(vmStatus)

	g.Expect(vmStatus.MarkedCompleted()).To(gomega.BeTrue())
	g.Expect(vmStatus.Phase).To(gomega.Equal(api.PhaseCompleted))
	g.Expect(vmStatus.HasCondition(api.ConditionFailed)).To(gomega.BeTrue())
	g.Expect(vmStatus.HasCondition(api.ConditionCanceled)).To(gomega.BeFalse())
	g.Expect(vmStatus.Error).NotTo(gomega.BeNil())
	g.Expect(vmStatus.Error.Reasons).To(gomega.ContainElement("VM was not found in inventory."))
	g.Expect(vmStatus.Pipeline[0].Error).NotTo(gomega.BeNil())
	g.Expect(vmStatus.Pipeline[0].Error.Reasons).To(gomega.ContainElement("VM was not found in inventory."))
}

func TestBuildInFlight_continuesAfterNotFound(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	missingRunning := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("missing-running")},
		Phase: api.PhaseStarted,
	}
	missingRunning.MarkStarted()

	running := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("running")},
		Phase: api.PhaseStarted,
	}
	running.MarkStarted()

	inv := &stubInventory{
		vms: map[string]model.VM{
			"running": inventoryVM("running", "host-a"),
		},
	}
	scheduler := newTestScheduler([]*plan.VMStatus{missingRunning, running}, inv)

	err := scheduler.buildInFlight()
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(missingRunning.MarkedCompleted()).To(gomega.BeTrue())
	g.Expect(missingRunning.HasCondition(api.ConditionFailed)).To(gomega.BeTrue())
	g.Expect(running.MarkedCompleted()).To(gomega.BeFalse())
	g.Expect(scheduler.inFlight["host-a"]).To(gomega.Equal(2))
}

func TestBuildPending_continuesAfterNotFound(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	missingQueued := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("missing-queued")},
		Phase: api.PhaseStarted,
	}
	queued := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("queued")},
		Phase: api.PhaseStarted,
	}

	inv := &stubInventory{
		vms: map[string]model.VM{
			"queued": inventoryVM("queued", "host-a"),
		},
	}
	scheduler := newTestScheduler([]*plan.VMStatus{missingQueued, queued}, inv)

	err := scheduler.buildPending()
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(missingQueued.MarkedCompleted()).To(gomega.BeTrue())
	g.Expect(missingQueued.HasCondition(api.ConditionCanceled)).To(gomega.BeTrue())
	g.Expect(scheduler.pending["host-a"]).To(gomega.HaveLen(1))
	g.Expect(scheduler.pending["host-a"][0].status.Ref.ID).To(gomega.Equal("queued"))
}

func TestHasActiveSharedDiskCreators_continuesAfterNotFound(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	migrateShared := true
	missingCreator := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("missing-creator")},
		Phase: api.PhaseStarted,
	}
	consumer := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("consumer")},
		Phase: api.PhaseStarted,
	}

	inv := &stubInventory{
		vms: map[string]model.VM{
			"consumer": inventoryVM("consumer", "host-a", true),
		},
	}
	scheduler := newTestScheduler([]*plan.VMStatus{missingCreator, consumer}, inv)
	scheduler.Plan.Spec.VMs = []plan.VM{
		{Ref: vmRef("missing-creator"), MigrateSharedDisks: &migrateShared},
		{Ref: vmRef("consumer")},
	}
	scheduler.Plan.Spec.MigrateSharedDisks = false

	active, err := scheduler.hasActiveSharedDiskCreators()
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(active).To(gomega.BeFalse())
	g.Expect(missingCreator.MarkedCompleted()).To(gomega.BeTrue())
	g.Expect(missingCreator.HasCondition(api.ConditionCanceled)).To(gomega.BeTrue())
}

func TestBuildPending_activeSharedDiskCreatorStillBlocks(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	migrateShared := true
	creator := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("creator")},
		Phase: api.PhaseStarted,
	}
	consumer := &plan.VMStatus{
		VM:    plan.VM{Ref: vmRef("consumer")},
		Phase: api.PhaseStarted,
	}

	inv := &stubInventory{
		vms: map[string]model.VM{
			"creator":  inventoryVM("creator", "host-a", true),
			"consumer": inventoryVM("consumer", "host-a", true),
		},
	}
	scheduler := newTestScheduler([]*plan.VMStatus{creator, consumer}, inv)
	scheduler.Plan.Spec.VMs = []plan.VM{
		{Ref: vmRef("creator"), MigrateSharedDisks: &migrateShared},
		{Ref: vmRef("consumer")},
	}
	scheduler.Plan.Spec.MigrateSharedDisks = false

	err := scheduler.buildPending()
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(scheduler.pending["host-a"]).To(gomega.HaveLen(1))
	g.Expect(scheduler.pending["host-a"][0].status.Ref.ID).To(gomega.Equal("creator"))
}
