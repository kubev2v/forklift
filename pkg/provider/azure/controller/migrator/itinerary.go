package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

const (
	PreHookFlag     = 1 << 0
	PostHookFlag    = 1 << 1
	ConversionFlag  = 1 << 2
	CrossRegionFlag = 1 << 3
	PreSnapshotFlag = 1 << 4
)

func (r *Migrator) Itinerary(vm planapi.VM) *libitr.Itinerary {
	r.vm = &vm

	itinerary := &libitr.Itinerary{
		Name: "Azure Cold Migration",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: PreHookFlag},
			{Name: api.PhaseStorePowerState},
			{Name: PhaseCreatePreSnapshot, All: PreSnapshotFlag},
			{Name: PhaseWaitForPreSnapshot, All: PreSnapshotFlag},
			{Name: PhaseDeallocateVM},
			{Name: PhaseWaitForDeallocation},
			{Name: PhaseCreateSnapshots},
			{Name: PhaseWaitForSnapshots},
			{Name: PhaseDeletePreSnapshots, All: PreSnapshotFlag},
			{Name: PhaseCopySnapshotsCrossRegion, All: CrossRegionFlag},
			{Name: PhaseWaitForCrossRegionSnapshots, All: CrossRegionFlag},
			{Name: PhaseCreateSnapshotContent},
			{Name: PhaseCreateVolumeSnapshot},
			{Name: PhaseCreatePVCs},
			{Name: PhaseWaitForPVCsBound},
			{Name: PhaseInjectOwnerRefs},
			{Name: api.PhaseCreateGuestConversionPod, All: ConversionFlag},
			{Name: api.PhaseConvertGuest, All: ConversionFlag},
			{Name: api.PhaseFinalize},
			{Name: api.PhaseCreateVM},
			{Name: api.PhasePostHook, All: PostHookFlag},
			{Name: api.PhaseCompleted},
		},
		Predicate: &AzurePredicate{
			vm:       &vm,
			context:  r.Context,
			migrator: r,
		},
	}

	return itinerary
}

type AzurePredicate struct {
	vm       *planapi.VM
	context  *plancontext.Context
	migrator *Migrator
}

// Evaluate decides whether a conditional pipeline phase should run.
// Each flag maps to a runtime condition (e.g. hooks configured, VM running,
// cross-region enabled). Phases without flags always run.
func (p *AzurePredicate) Evaluate(flag libitr.Flag) (bool, error) {
	if p.vm == nil {
		return false, nil
	}

	if flag&PreHookFlag != 0 {
		_, found := p.vm.FindHook(api.PhasePreHook)
		return found, nil
	}

	if flag&PostHookFlag != 0 {
		_, found := p.vm.FindHook(api.PhasePostHook)
		return found, nil
	}

	if flag&ConversionFlag != 0 {
		return p.context.Source.Provider.RequiresConversion() && !p.context.Plan.Spec.SkipGuestConversion, nil
	}

	if flag&CrossRegionFlag != 0 {
		if p.migrator != nil {
			return p.migrator.getAzureClient().IsCrossRegion(), nil
		}
		return false, nil
	}

	if flag&PreSnapshotFlag != 0 {
		if p.migrator != nil {
			running, err := p.migrator.isVMRunning()
			return running, err
		}
		return false, nil
	}

	return true, nil
}

// Count returns the number of distinct predicate flags used in the pipeline.
func (p *AzurePredicate) Count() int {
	return 5
}
