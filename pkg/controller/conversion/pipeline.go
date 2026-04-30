package conversion

import (
	"context"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// SpecSettingsSnapshotMorefKey is the settings key for a user-supplied snapshot MoRef.
// When set the controller skips snapshot creation/removal and does not own the snapshot.
const SpecSettingsSnapshotMorefKey = "SNAPSHOT_MOREF"

// PipelineStage is a single entry in a ConversionPipeline stage sequence.
// When Predicate is non-nil and returns false for the current Conversion the stage
// is automatically skipped and pipeline advances to the next stage without
// calling the stage handler.
type PipelineStage struct {
	Stage     api.ConversionStage
	Predicate func(conv *api.Conversion) bool
}

// snapshotOwnedByController returns true when no SNAPSHOT_MOREF has been
// supplied in spec.settings, meaning the controller is responsible for creating
// and removing the snapshot itself.
func snapshotOwnedByController(conv *api.Conversion) bool {
	return strings.TrimSpace(conv.Spec.Settings[SpecSettingsSnapshotMorefKey]) == ""
}

// VirtV2vPipelineStages is the ordered list of stages for Inspection / InPlace / Remote workloads.
var VirtV2vPipelineStages = []PipelineStage{
	{Stage: api.StageCreatePod},
	{Stage: api.StagePodRunning},
	{Stage: api.StageFinished},
}

// DeepInspectionPipelineStages is the ordered list of stages for DeepInspection workloads.
// Stages that have a Predicate are skipped when the predicate returns false
var DeepInspectionPipelineStages = []PipelineStage{
	{Stage: api.StageCreateSnapshot, Predicate: snapshotOwnedByController},
	{Stage: api.StageWaitForSnapshot, Predicate: snapshotOwnedByController},
	{Stage: api.StageCreatePod},
	{Stage: api.StagePodRunning},
	{Stage: api.StageRemoveSnapshot, Predicate: snapshotOwnedByController},
	{Stage: api.StageWaitForSnapshotRemoval, Predicate: snapshotOwnedByController},
	{Stage: api.StageFinished},
}

// ConversionPipeline drives reconciliation for a single Conversion CR.
type ConversionPipeline struct {
	ctx    context.Context
	r      *Reconciler
	conv   *api.Conversion
	stages []PipelineStage
}

// NewConversionPipeline builds a pipeline for the conversion type using the matching stage definition.
func NewConversionPipeline(ctx context.Context, cr *Reconciler, conv *api.Conversion) *ConversionPipeline {
	switch conv.Spec.Type {
	case api.DeepInspection:
		return &ConversionPipeline{ctx: ctx, r: cr, conv: conv, stages: DeepInspectionPipelineStages}
	default:
		return &ConversionPipeline{ctx: ctx, r: cr, conv: conv, stages: VirtV2vPipelineStages}
	}
}

// advanceStage sets conv.Status.Stage to the next. no-op if the stage is not found or is already the last stage.
func (p *ConversionPipeline) advanceStage() {
	for i, s := range p.stages {
		if s.Stage == p.conv.Status.Stage && i+1 < len(p.stages) {
			p.conv.Status.Stage = p.stages[i+1].Stage
			return
		}
	}
}

// currentStage returns the PipelineStage matching conv.Status.Stage, or nil.
func (p *ConversionPipeline) currentStage() *PipelineStage {
	for i := range p.stages {
		if p.stages[i].Stage == p.conv.Status.Stage {
			return &p.stages[i]
		}
	}
	return nil
}

// checkStagePredicate advances past consecutive stages whose Predicate returns
// false.
func (p *ConversionPipeline) checkStagePredicate() {
	for {
		stage := p.currentStage()
		if stage == nil || stage.Predicate == nil || stage.Predicate(p.conv) {
			break
		}
		p.advanceStage()
	}
}

// Run executes one reconcile step and returns the conversion phase that was reached.
// When the phase is Pending it first runs runPhasePending, which transitions the
// conversion to Running or Failed. If the transition
// succeeds, pipeline immediately executes the first stage in the same reconcile
// cycle to avoid an extra reconcile.
// Run executes one reconcile step and returns (true, nil) when all pipeline
// stages have completed and the pod succeeded.
// Returns (false, nil) while work is still in progress
func (p *ConversionPipeline) Run() (succeeded bool, err error) {
	if p.conv.Status.Phase == api.PhasePending {
		err = p.runPhasePending()
		if err != nil {
			return false, err
		}
		// when not running, there was an error with the provider
		if p.conv.Status.Phase != api.PhaseRunning {
			return false, nil
		}
	}

	switch p.conv.Spec.Type {
	case api.DeepInspection:
		return p.runDeepInspection()
	default:
		return p.runVirtV2v()
	}
}

// For DeepInspection it resolves snapshot ownership. Supplied SNAPSHOT_MOREF
// in spec.settings means the controller does not own the snapshot
func (p *ConversionPipeline) runPhasePending() error {
	if p.conv.Spec.Type == api.DeepInspection {
		snapshotMoref := strings.TrimSpace(p.conv.Spec.Settings[SpecSettingsSnapshotMorefKey])
		if snapshotMoref != "" {
			p.conv.Status.Snapshot = &api.SnapshotStatus{Moref: snapshotMoref, Owned: false}
		} else {
			provider, _, err := p.r.loadProviderSecret(p.ctx, p.conv)
			if err != nil {
				return err
			}
			if provider.Type() != api.VSphere {
				p.conv.Status.Phase = api.PhaseFailed
				p.conv.Status.SetCondition(libcnd.Condition{
					Type:     "NonVSphereProvider",
					Status:   True,
					Category: Critical,
					Message:  "DeepInspection requires SNAPSHOT_MOREF in spec.settings or a vSphere provider to create a snapshot.",
				})
				return nil
			}
			p.conv.Status.Snapshot = &api.SnapshotStatus{Owned: true}
		}
	}

	now := meta.Now()
	p.conv.Status.Phase = api.PhaseRunning
	p.conv.Status.StartTime = &now
	return nil
}

func (p *ConversionPipeline) runVirtV2v() (pipelineFinished bool, err error) {
	if p.conv.Status.Stage == "" {
		p.conv.Status.Stage = p.stages[0].Stage
	}

	// currently, this does nothing for virt-v2v pipeline because it has no predicates, only here for consistency.
	p.checkStagePredicate()

	var stageDone bool
	switch p.conv.Status.Stage {
	case api.StageCreatePod, api.StagePodRunning:
		stageDone, err = p.runStageEnsurePod()
		if err != nil {
			return false, err
		}
		if stageDone {
			p.advanceStage()
		}
	case api.StageFinished:
		return p.podSucceeded()
	default:
		return false, liberr.New("unknown stage", "stage", p.conv.Status.Stage)
	}
	return false, nil
}

func (p *ConversionPipeline) runDeepInspection() (pipelineFinished bool, err error) {
	if p.conv.Status.Snapshot == nil {
		p.conv.Status.Snapshot = &api.SnapshotStatus{}
	}

	if p.conv.Status.Stage == "" {
		p.conv.Status.Stage = p.stages[0].Stage
	}

	p.checkStagePredicate()

	var stageDone bool
	switch p.conv.Status.Stage {
	case api.StageCreateSnapshot:
		stageDone, err = p.runStageCreatingSnapshot()
	case api.StageWaitForSnapshot:
		stageDone, err = p.runStageWaitingForSnapshot()
	case api.StageCreatePod, api.StagePodRunning:
		stageDone, err = p.runStageEnsurePod()
	case api.StageRemoveSnapshot:
		stageDone, err = p.runStageRemovingSnapshot()
	case api.StageWaitForSnapshotRemoval:
		stageDone, err = p.runStageWaitingForSnapshotRemoval()
	case api.StageFinished:
		return p.podSucceeded()
	}

	if err != nil {
		return false, err
	}
	if stageDone {
		p.advanceStage()
	}
	return false, nil
}

// runStageCreatingSnapshot submits the vSphere snapshot creation task
func (p *ConversionPipeline) runStageCreatingSnapshot() (stageDone bool, err error) {
	snap := p.conv.Status.Snapshot
	provider, secret, err := p.r.loadProviderSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}
	if provider.Type() != api.VSphere {
		p.conv.Status.Phase = api.PhaseFailed
		return stageDone, nil
	}

	gClient, err := GovmomiClientFromProvider(p.ctx, provider, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, provider, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	_, taskID, err := snapClient.CreateSnapshot()
	if err != nil {
		return stageDone, err
	}
	snap.CreateTaskID = taskID
	return true, nil
}

// runStageWaitingForSnapshot polls the snapshot creation task until the MoRef is available
func (p *ConversionPipeline) runStageWaitingForSnapshot() (stageDone bool, err error) {
	snap := p.conv.Status.Snapshot
	provider, secret, err := p.r.loadProviderSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}

	gClient, err := GovmomiClientFromProvider(p.ctx, provider, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, provider, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	ready, moref, err := snapClient.CheckCreateTaskReady(snap.CreateTaskID)
	if err != nil {
		p.conv.Status.Phase = api.PhaseFailed
		return stageDone, nil
	}
	if !ready {
		return stageDone, nil
	}
	snap.CreateTaskID = ""
	snap.Moref = moref
	return true, nil
}

// runStageEnsurePod ensures the conversion pod exists and tracks its progress.
func (p *ConversionPipeline) runStageEnsurePod() (stageDone bool, err error) {
	ensurer, err := NewEnsurer(p.r.Client, p.r.Log, p.conv.Spec)
	if err != nil {
		return stageDone, err
	}
	pod, err := ensurer.EnsurePod(p.conv)
	if err != nil {
		return stageDone, err
	}
	if pod == nil {
		return stageDone, nil
	}

	p.conv.Status.Pod = core.ObjectReference{Namespace: pod.Namespace, Name: pod.Name}

	switch pod.Status.Phase {
	case core.PodPending:
		p.conv.Status.Stage = api.StageCreatePod
		return stageDone, nil
	case core.PodRunning:
		p.conv.Status.Stage = api.StagePodRunning
		return stageDone, nil
	}

	// Pod exited
	p.conv.Status.Stage = api.StagePodRunning
	return true, nil
}

// podSucceeded is called by the pipeline runners when StageFinished is reached.
// It fetches the pod and returns true when it succeeded.
func (p *ConversionPipeline) podSucceeded() (podSucceeded bool, err error) {
	ensurer, err := NewEnsurer(p.r.Client, p.r.Log, p.conv.Spec)
	if err != nil {
		return false, err
	}
	pod, err := ensurer.EnsurePod(p.conv)
	if err != nil {
		return false, err
	}
	return pod != nil && pod.Status.Phase == core.PodSucceeded, nil
}

// runStageRemovingSnapshot submits the vSphere snapshot removal task
func (p *ConversionPipeline) runStageRemovingSnapshot() (stageDone bool, err error) {
	snap := p.conv.Status.Snapshot
	provider, secret, err := p.r.loadProviderSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}

	gClient, err := GovmomiClientFromProvider(p.ctx, provider, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, provider, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	taskID, err := snapClient.RemoveSnapshot(snap.Moref)
	if err != nil {
		return stageDone, err
	}
	snap.RemoveTaskID = taskID
	return true, nil
}

// runStageWaitingForSnapshotRemoval polls the snapshot removal task until it
// completes
func (p *ConversionPipeline) runStageWaitingForSnapshotRemoval() (stageDone bool, err error) {
	snap := p.conv.Status.Snapshot
	provider, secret, err := p.r.loadProviderSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}

	gClient, err := GovmomiClientFromProvider(p.ctx, provider, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, provider, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	ready, err := snapClient.CheckRemoveTaskReady(snap.RemoveTaskID)
	if err != nil {
		p.conv.Status.Phase = api.PhaseFailed
		return stageDone, nil
	}
	if !ready {
		return stageDone, nil
	}

	// Snapshot removed
	p.conv.Status.Snapshot = nil
	return true, nil
}

// loadProviderSecret returns the source Provider and its credentials Secret.
func (r *Reconciler) loadProviderSecret(ctx context.Context, conversion *api.Conversion) (*api.Provider, *core.Secret, error) {
	provider := &api.Provider{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: conversion.Spec.Provider.Namespace,
		Name:      conversion.Spec.Provider.Name,
	}, provider)
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	secret := &core.Secret{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: provider.Spec.Secret.Namespace,
		Name:      provider.Spec.Secret.Name,
	}, secret)
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	return provider, secret, nil
}
