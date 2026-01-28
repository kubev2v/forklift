package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

// Predicate flags for conditional phase execution.
const (
	PreHookFlag      = 1 << 0 // Include pre-hook phase
	PostHookFlag     = 1 << 1 // Include post-hook phase
	ConversionFlag   = 1 << 2 // Include guest conversion phases
	CrossAccountFlag = 1 << 3 // Include cross-account snapshot sharing phase
)

// Itinerary builds the EC2 cold migration workflow sequence defining phase order.
// Includes: Initialize→PreHook→PowerOff→CreateSnapshots→WaitSnapshots→[ShareSnapshots]→CreateVolumes→WaitForVolumes→CreatePVsAndPVCs→CreateGuestConversionPod→ConvertGuest→Finalize→CreateVM→RemoveSnapshots→PostHook→Completed.
// Pre/post hooks are conditionally included based on VM hook configuration.
// ShareSnapshots phase is conditionally included for cross-account migrations.
func (r *Migrator) Itinerary(vm planapi.VM) *libitr.Itinerary {
	r.vm = &vm

	itinerary := &libitr.Itinerary{
		Name: "EC2 Cold Migration",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: PreHookFlag},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: PhaseCreateSnapshots},
			{Name: PhaseWaitForSnapshots},
			{Name: PhaseShareSnapshots, All: CrossAccountFlag},
			{Name: PhaseCreateVolumes},
			{Name: PhaseWaitForVolumes},
			{Name: PhaseCreatePVsAndPVCs},
			{Name: api.PhaseCreateGuestConversionPod, All: ConversionFlag},
			{Name: api.PhaseConvertGuest, All: ConversionFlag},
			{Name: api.PhaseFinalize},
			{Name: api.PhaseCreateVM},
			{Name: PhaseRemoveSnapshots},
			{Name: api.PhasePostHook, All: PostHookFlag},
			{Name: api.PhaseCompleted},
		},
		Predicate: &EC2Predicate{
			vm:       &vm,
			context:  r.Context,
			migrator: r,
		},
	}

	return itinerary
}

// EC2Predicate implements conditional phase evaluation for the EC2 migration itinerary.
type EC2Predicate struct {
	vm       *planapi.VM
	context  *plancontext.Context
	migrator *Migrator
}

func (p *EC2Predicate) Evaluate(flag libitr.Flag) (bool, error) {
	if p.vm == nil {
		return false, nil
	}

	// Pre-hook phase: include if VM has a pre-hook configured
	if flag&PreHookFlag != 0 {
		_, found := p.vm.FindHook(api.PhasePreHook)
		return found, nil
	}

	// Post-hook phase: include if VM has a post-hook configured
	if flag&PostHookFlag != 0 {
		_, found := p.vm.FindHook(api.PhasePostHook)
		return found, nil
	}

	// Guest conversion phases: include if provider requires conversion and not skipped
	if flag&ConversionFlag != 0 {
		return p.context.Source.Provider.RequiresConversion() && !p.context.Plan.Spec.SkipGuestConversion, nil
	}

	// Cross-account snapshot sharing: include if cross-account mode is enabled
	if flag&CrossAccountFlag != 0 {
		if p.migrator != nil {
			ec2Client := p.migrator.getEC2Client()
			return ec2Client.IsCrossAccount(), nil
		}
		return false, nil
	}

	return true, nil
}

func (p *EC2Predicate) Count() int {
	return 4 // PreHook, PostHook, Conversion, CrossAccount
}
