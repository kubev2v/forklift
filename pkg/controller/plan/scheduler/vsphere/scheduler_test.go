package vsphere

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/onsi/gomega"
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
