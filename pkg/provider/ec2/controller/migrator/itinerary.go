package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

// Itinerary builds the EC2 cold migration workflow sequence defining phase order.
// Includes: Initialize→PreHook→PowerOff→CreateSnapshots→WaitSnapshots→CreateDataVolumes→Finalize→CreateVM→RemoveSnapshots→PostHook→Completed.
// Pre/post hooks are conditionally included based on VM hook configuration (evaluated via EC2Predicate).
func (r *Migrator) Itinerary(vm planapi.VM) *libitr.Itinerary {
	r.vm = &vm

	itinerary := &libitr.Itinerary{
		Name: "EC2 Cold Migration",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: 1 << 0},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: PhaseCreateSnapshots},
			{Name: PhaseWaitForSnapshots},
			{Name: api.PhaseCreateDataVolumes},
			{Name: api.PhaseFinalize},
			{Name: api.PhaseCreateVM},
			{Name: PhaseRemoveSnapshots},
			{Name: api.PhasePostHook, All: 1 << 1},
			{Name: api.PhaseCompleted},
		},
		Predicate: &EC2Predicate{vm: &vm},
	}

	return itinerary
}

// EC2Predicate implements hook predicate evaluation.
type EC2Predicate struct {
	vm *planapi.VM
}

func (p *EC2Predicate) Evaluate(flag libitr.Flag) (bool, error) {
	if p.vm == nil {
		return false, nil
	}

	if flag&(1<<0) != 0 {
		_, found := p.vm.FindHook(api.PhasePreHook)
		return found, nil
	}

	if flag&(1<<1) != 0 {
		_, found := p.vm.FindHook(api.PhasePostHook)
		return found, nil
	}

	return true, nil
}

func (p *EC2Predicate) Count() int {
	return 2
}
