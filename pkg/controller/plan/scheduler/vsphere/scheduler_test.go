package vsphere

import (
	"github.com/onsi/gomega"
	"testing"
)

func TestScheduler(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	hostA := "hostA"
	hostB := "hostB"
	hostC := "hostC"

	scheduler := Scheduler{MaxInFlight: 10}
	scheduler.inflight = map[string]int{
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
