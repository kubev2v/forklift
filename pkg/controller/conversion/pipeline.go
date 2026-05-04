package conversion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
	return strings.TrimSpace(conv.Spec.Settings[api.SpecSettingsSnapshotMorefKey]) == ""
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
	{Stage: api.StageFetchingResults},
	{Stage: api.StageRemoveSnapshot, Predicate: snapshotOwnedByController},
	{Stage: api.StageWaitForSnapshotRemoval, Predicate: snapshotOwnedByController},
	{Stage: api.StageFinished},
}

// inspectionResultPort is the port the deep-inspection pod's HTTP server listens on.
const inspectionResultPort = 8080

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
			next := p.stages[i+1].Stage
			p.r.Log.Info("Stage transition.", "from", p.conv.Status.Stage, "to", next)
			p.conv.Status.Stage = next
			return
		}
	}
}

// setPhase updates the conversion phase and logs the transition.
func (p *ConversionPipeline) setPhase(phase api.ConversionPhase) {
	p.r.Log.Info("Phase transition.", "from", p.conv.Status.Phase, "to", phase)
	p.conv.Status.Phase = phase
}

// setStage updates the conversion stage and logs when the value actually changes.
func (p *ConversionPipeline) setStage(stage api.ConversionStage) {
	if p.conv.Status.Stage == stage {
		return
	}
	p.r.Log.Info("Stage transition.", "from", p.conv.Status.Stage, "to", stage)
	p.conv.Status.Stage = stage
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
		p.r.Log.V(3).Info("Skipping stage (predicate false).", "stage", stage.Stage)
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
	p.r.Log.V(3).Info("Pipeline run.",
		"type", p.conv.Spec.Type,
		"phase", p.conv.Status.Phase,
		"stage", p.conv.Status.Stage)

	if p.conv.Status.Phase == api.PhasePending {
		err = p.runPhasePending()
		if err != nil {
			p.r.Log.Error(err, "runPhasePending failed.")
			return false, err
		}
		// when not running, there was an error with the provider
		if p.conv.Status.Phase != api.PhaseRunning {
			p.r.Log.Info("Conversion did not transition to Running after pending phase.",
				"phase", p.conv.Status.Phase)
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
		snapshotMoref := strings.TrimSpace(p.conv.Spec.Settings[api.SpecSettingsSnapshotMorefKey])
		p.r.Log.Info("Pending phase: resolving snapshot ownership.",
			"snapshotMorefProvided", snapshotMoref != "")
		if snapshotMoref != "" {
			p.r.Log.Info("Using supplied snapshot MoRef; controller will not own the snapshot.",
				"snapshotMoref", snapshotMoref)
			p.conv.Status.Snapshot = &api.SnapshotStatus{Moref: snapshotMoref, Owned: false}
		} else {
			if p.conv.Spec.Connection.Secret.Name == "" {
				p.r.Log.Info("Connection secret not set; cannot own snapshot.")
				p.setPhase(api.PhaseFailed)
				p.conv.Status.SetCondition(libcnd.Condition{
					Type:     "ConnectionSecretNotSet",
					Status:   True,
					Category: Critical,
					Message:  "DeepInspection requires a Connection secret when the controller owns snapshots.",
				})
				return nil
			}
			p.conv.Status.Snapshot = &api.SnapshotStatus{Owned: true}
		}
	}

	now := meta.Now()
	p.setPhase(api.PhaseRunning)
	p.conv.Status.StartTime = &now
	return nil
}

func (p *ConversionPipeline) runVirtV2v() (pipelineFinished bool, err error) {
	if p.conv.Status.Stage == "" {
		p.setStage(p.stages[0].Stage)
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
		p.setStage(p.stages[0].Stage)
	}

	p.checkStagePredicate()

	p.r.Log.V(3).Info("Deep inspection stage.", "stage", p.conv.Status.Stage)

	var stageDone bool
	switch p.conv.Status.Stage {
	case api.StageCreateSnapshot:
		stageDone, err = p.runStageCreatingSnapshot()
	case api.StageWaitForSnapshot:
		stageDone, err = p.runStageWaitingForSnapshot()
	case api.StageCreatePod, api.StagePodRunning:
		stageDone, err = p.runStageDeepInspectionPod()
	case api.StageFetchingResults:
		stageDone, err = p.runStageFetchingResults()
	case api.StageRemoveSnapshot:
		stageDone, err = p.runStageRemovingSnapshot()
	case api.StageWaitForSnapshotRemoval:
		stageDone, err = p.runStageWaitingForSnapshotRemoval()
	case api.StageFinished:
		return p.podSucceeded()
	}

	if err != nil {
		p.r.Log.Error(err, "Stage failed.", "stage", p.conv.Status.Stage)
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
	secret, err := p.r.loadConnectionSecret(p.ctx, p.conv)
	if err != nil {
		p.r.Log.Error(err, "Failed to load connection secret for snapshot creation.")
		return stageDone, err
	}

	gClient, err := GovmomiClientFromSecret(p.ctx, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	_, taskID, err := snapClient.CreateSnapshot()
	if err != nil {
		p.r.Log.Error(err, "CreateSnapshot failed.")
		return stageDone, err
	}
	p.r.Log.Info("Snapshot creation task submitted.", "taskID", taskID)
	snap.CreateTaskID = taskID
	return true, nil
}

// runStageWaitingForSnapshot polls the snapshot creation task until the MoRef is available
func (p *ConversionPipeline) runStageWaitingForSnapshot() (stageDone bool, err error) {
	snap := p.conv.Status.Snapshot
	secret, err := p.r.loadConnectionSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}

	gClient, err := GovmomiClientFromSecret(p.ctx, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	ready, moref, err := snapClient.CheckCreateTaskReady(snap.CreateTaskID)
	if err != nil {
		p.r.Log.Error(err, "Snapshot creation task failed.", "taskID", snap.CreateTaskID)
		p.setPhase(api.PhaseFailed)
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
		p.setStage(api.StageCreatePod)
		return stageDone, nil
	case core.PodRunning:
		p.setStage(api.StagePodRunning)
		return stageDone, nil
	}

	// Pod exited
	p.setStage(api.StagePodRunning)
	return true, nil
}

// runStageDeepInspectionPod ensures the deep-inspection pod exists and tracks
// its progress.Unlike the generic runStageEnsurePod it also polls the pod's
// HTTP /ready endpoint so the pipeline can advance to StageFetchingResults
// while the pod is still running and serving results.
func (p *ConversionPipeline) runStageDeepInspectionPod() (stageDone bool, err error) {
	ensurer, err := NewEnsurer(p.r.Client, p.r.Log, p.conv.Spec)
	if err != nil {
		p.r.Log.Error(err, "Failed to create ensurer for deep inspection pod.")
		return
	}
	pod, err := ensurer.EnsurePod(p.conv)
	if err != nil {
		p.r.Log.Error(err, "EnsurePod failed.")
		return
	}
	if pod == nil {
		p.r.Log.V(3).Info("EnsurePod returned nil; waiting.")
		return
	}

	p.conv.Status.Pod = core.ObjectReference{Namespace: pod.Namespace, Name: pod.Name}
	p.r.Log.V(3).Info("Deep inspection pod status.", "pod", pod.Name, "phase", pod.Status.Phase, "podIP", pod.Status.PodIP)

	switch pod.Status.Phase {
	case core.PodPending:
		p.setStage(api.StageCreatePod)
		return
	case core.PodRunning:
		p.setStage(api.StagePodRunning)
		// Advance early when the pod signals that detection is complete.
		if p.isResultReady(pod.Status.PodIP) {
			p.r.Log.Info("Deep inspection pod is ready; advancing to FetchingResults.")
			return true, nil
		}
		return
	}

	// Pod has exited (Succeeded, Failed, Unknown) — advance regardless so
	// the pipeline can still attempt result fetching or skip gracefully.
	p.r.Log.Info("Deep inspection pod exited.", "pod", pod.Name, "phase", pod.Status.Phase)
	p.setStage(api.StagePodRunning)
	return true, nil
}

// isResultReady probes GET /ready on the deep-inspection pod.  Returns true only
// when the pod responds with 200 OK, indicating that vmdetect.Detect has
// completed and results are ready to be served.
func (p *ConversionPipeline) isResultReady(podIP string) bool {
	if podIP == "" {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/ready", podIP, inspectionResultPort))
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// runStageFetchingResults fetches the DetectResult JSON from the deep-inspection
// pod's HTTP /results endpoint and stores a summary in conv.Status.InspectionResult.
// If the pod has already exited before results could be fetched, the stage is
// skipped gracefully so the pipeline can continue.
func (p *ConversionPipeline) runStageFetchingResults() (stageDone bool, err error) {
	podRef := p.conv.Status.Pod
	if podRef.Name == "" {
		p.r.Log.Info("No pod reference in status; skipping result fetch.")
		return true, nil
	}

	pod := &core.Pod{}
	err = p.r.Get(p.ctx, types.NamespacedName{
		Namespace: podRef.Namespace,
		Name:      podRef.Name,
	}, pod)
	if err != nil {
		p.r.Log.Error(err, "Failed to get deep inspection pod.", "pod", podRef.Name)
		return
	}

	p.r.Log.V(3).Info("Fetching results: pod status.", "pod", pod.Name, "phase", pod.Status.Phase, "podIP", pod.Status.PodIP)

	// Pod exited before we could fetch results, return error
	if pod.Status.Phase != core.PodRunning {
		return false, fmt.Errorf("pod %s exited before /results could be fetched: phase=%s", pod.Name, pod.Status.Phase)
	}
	if pod.Status.PodIP == "" {
		p.r.Log.V(3).Info("Pod has no IP yet; retrying.")
		return
	}

	result, fetchErr := p.fetchInspectionResults(pod.Status.PodIP)
	if fetchErr != nil {
		// Transient connection error, retry
		p.r.Log.V(3).Info("Transient error fetching results; retrying.", "error", fetchErr.Error())
		return
	}
	if result == nil {
		// Pod returned 503, not ready yet
		p.r.Log.V(3).Info("Results not ready yet (503); retrying.")
		return
	}

	p.r.Log.Info("Inspection results fetched.", "allChecksPassed", result.AllChecksPassed, "concerns", len(result.Concerns))
	p.conv.Status.InspectionResult = result
	return true, nil
}

// fetchInspectionResults calls GET /results on the deep-inspection pod and
// maps the response into the API InspectionResult type.
// Returns (nil, nil) when the pod responds with 503 (not ready).
func (p *ConversionPipeline) fetchInspectionResults(podIP string) (*api.InspectionResult, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/results", podIP, inspectionResultPort))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from deep-inspection /results", resp.StatusCode)
	}

	// Decode only the subset of fields we persist on the CR.
	var raw struct {
		AllChecksPassed bool `json:"all_checks_passed"`
		AllConcerns     []struct {
			ID       string `json:"id"`
			Category string `json:"category"`
			Label    string `json:"label"`
			Message  string `json:"message"`
		} `json:"all_concerns"`
		OSInfo *struct {
			Name         string `json:"name"`
			Distro       string `json:"distro"`
			MajorVersion string `json:"major_version"`
			Architecture string `json:"architecture"`
		} `json:"os_info"`
		Filesystems []struct {
			Device string `json:"device"`
			Type   string `json:"type"`
			UUID   string `json:"uuid"`
		} `json:"filesystems"`
		Mountpoints []struct {
			Device     string `json:"device"`
			MountPoint string `json:"mount_point"`
		} `json:"mountpoints"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	result := &api.InspectionResult{AllChecksPassed: raw.AllChecksPassed}
	if raw.OSInfo != nil {
		result.OSInfo = &api.OSInfo{
			Name:    raw.OSInfo.Name,
			Distro:  raw.OSInfo.Distro,
			Version: raw.OSInfo.MajorVersion,
			Arch:    raw.OSInfo.Architecture,
		}
	}
	for _, c := range raw.AllConcerns {
		result.Concerns = append(result.Concerns, api.InspectionConcern{
			ID:       c.ID,
			Category: c.Category,
			Label:    c.Label,
			Message:  c.Message,
		})
	}
	for _, f := range raw.Filesystems {
		result.Filesystems = append(result.Filesystems, api.InspectionFilesystem{
			Device: f.Device,
			Type:   f.Type,
			UUID:   f.UUID,
		})
	}
	for _, m := range raw.Mountpoints {
		result.Mountpoints = append(result.Mountpoints, api.InspectionMountpoint{
			Device:     m.Device,
			MountPoint: m.MountPoint,
		})
	}
	return result, nil
}

// podSucceeded is called by the pipeline runners when StageFinished is reached.
// Returns (true, nil) when the pod succeeded, (false, err) when it failed, and
// (false, nil) when it is still running or not yet found.
func (p *ConversionPipeline) podSucceeded() (podSucceeded bool, err error) {
	ensurer, err := NewEnsurer(p.r.Client, p.r.Log, p.conv.Spec)
	if err != nil {
		p.r.Log.Error(err, "Failed to create ensurer in podSucceeded.")
		return false, err
	}
	pod, err := ensurer.EnsurePod(p.conv)
	if err != nil {
		p.r.Log.Error(err, "EnsurePod failed in podSucceeded.")
		return false, err
	}
	if pod == nil {
		p.r.Log.Info("podSucceeded: pod not found.")
		return false, nil
	}
	p.r.Log.Info("podSucceeded: pod phase check.", "pod", pod.Name, "phase", pod.Status.Phase)
	switch pod.Status.Phase {
	case core.PodSucceeded:
		return true, nil
	case core.PodFailed:
		return false, liberr.New("conversion pod failed", "pod", pod.Name, "phase", pod.Status.Phase)
	case core.PodUnknown:
		return false, liberr.New("conversion pod in unknown state", "pod", pod.Name)
	default:
		// PodPending/PodRunning, still in progress
		return false, nil
	}
}

// runStageRemovingSnapshot submits the vSphere snapshot removal task
func (p *ConversionPipeline) runStageRemovingSnapshot() (stageDone bool, err error) {
	snap := p.conv.Status.Snapshot
	secret, err := p.r.loadConnectionSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}

	gClient, err := GovmomiClientFromSecret(p.ctx, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, p.conv.Spec.VM)
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
	secret, err := p.r.loadConnectionSecret(p.ctx, p.conv)
	if err != nil {
		return stageDone, err
	}

	gClient, err := GovmomiClientFromSecret(p.ctx, secret)
	if err != nil {
		return stageDone, err
	}
	defer func() {
		_ = gClient.Logout(p.ctx)
		gClient.CloseIdleConnections()
	}()

	snapClient, err := NewSnapshotClient(p.r.Log, gClient, p.conv.Spec.VM)
	if err != nil {
		return stageDone, err
	}

	ready, err := snapClient.CheckRemoveTaskReady(snap.RemoveTaskID)
	if err != nil {
		p.r.Log.Error(err, "Snapshot removal task failed.", "taskID", snap.RemoveTaskID)
		p.setPhase(api.PhaseFailed)
		return stageDone, nil
	}
	if !ready {
		return stageDone, nil
	}

	// Snapshot removed
	p.conv.Status.Snapshot = nil
	return true, nil
}

// loadConnectionSecret returns the Secret referenced by conv.Spec.Connection.Secret.
func (r *Reconciler) loadConnectionSecret(ctx context.Context, conversion *api.Conversion) (*core.Secret, error) {
	if conversion.Spec.Connection.Secret.Name == "" {
		return nil, liberr.New("Connection.Secret not set on Conversion CR")
	}
	if conversion.Spec.Connection.Secret.Namespace == "" {
		return nil, liberr.New("Connection.Secret.Namespace not set on Conversion CR")
	}
	secret := &core.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: conversion.Spec.Connection.Secret.Namespace,
		Name:      conversion.Spec.Connection.Secret.Name,
	}, secret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return secret, nil
}
