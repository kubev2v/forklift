package conversion

import (
	"context"
	"errors"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name = "conversion"
)

var log = logging.WithName(Name)

var Settings = &settings.Settings

// Creates a new Conversion Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		Reconciler: base.Reconciler{
			EventRecorder: mgr.GetEventRecorderFor(Name),
			Client:        mgr.GetClient(),
			Log:           log,
		},
	}
	cnt, err := controller.New(
		Name,
		mgr,
		controller.Options{
			Reconciler:              reconciler,
			MaxConcurrentReconciles: Settings.MaxConcurrentReconciles,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	err = cnt.Watch(
		source.Kind(
			mgr.GetCache(),
			&api.Conversion{},
			&handler.TypedEnqueueRequestForObject[*api.Conversion]{},
			&ConversionPredicate{}))
	if err != nil {
		log.Trace(err)
		return err
	}
	err = cnt.Watch(
		source.Kind(
			mgr.GetCache(),
			&core.Pod{},
			handler.TypedEnqueueRequestsFromMapFunc(conversionRequestsForPod),
			predicate.TypedFuncs[*core.Pod]{
				CreateFunc: func(e event.TypedCreateEvent[*core.Pod]) bool {
					return conversionPodEvent(e.Object)
				},
				UpdateFunc: func(e event.TypedUpdateEvent[*core.Pod]) bool {
					return conversionPodEvent(e.ObjectNew) &&
						e.ObjectOld.Status.Phase != e.ObjectNew.Status.Phase
				},
				DeleteFunc:  func(event.TypedDeleteEvent[*core.Pod]) bool { return false },
				GenericFunc: func(event.TypedGenericEvent[*core.Pod]) bool { return false },
			}))
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// conversionPodEvent reports whether a pod update should trigger conversion reconciliation.
func conversionPodEvent(pod *core.Pod) bool {
	return pod != nil && pod.Labels[convctx.LabelConversion] != ""
}

// conversionRequestsForPod maps a labeled conversion pod to its Conversion reconcile request.
func conversionRequestsForPod(_ context.Context, pod *core.Pod) []reconcile.Request {
	if pod == nil {
		return nil
	}
	name := pod.Labels[convctx.LabelConversion]
	if name == "" {
		return nil
	}
	ns := pod.Labels[convctx.LabelPlanNamespace]
	if ns == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}}}
}

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	base.Reconciler
}

// Reconcile a Conversion CR.
// Note: Must not be a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"conversion",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	conversion := &api.Conversion{}
	err = r.Get(ctx, request.NamespacedName, conversion)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("Conversion deleted.")
			err = nil
		}
		return
	}
	defer func() {
		r.Log.V(2).Info("Conditions.", "all", conversion.Status.Conditions)
	}()

	if conversion.Status.Phase == api.PhaseSucceeded {
		return
	}
	if conversion.Status.Phase == api.PhaseFailed {
		recovered, requeue, recErr := r.reconcileFailedConversion(ctx, conversion)
		if recErr != nil {
			err = recErr
			return
		}
		if recovered {
			return
		}
		if requeue {
			result.RequeueAfter = base.SlowReQ
		}
		return
	}

	if conversion.Status.Phase == "" {
		conversion.Status.Phase = api.PhasePending
	}

	conversion.Status.BeginStagingConditions()

	// Validate the spec.
	err = r.validate(conversion)
	if err != nil {
		return
	}

	// Canceled: delete pod, trigger snapshot removal (fire-and-forget), keep secrets.
	if conversion.Status.Phase == api.PhaseCanceled {
		ensurer, ensureErr := NewEnsurer(r.Client, r.Log, conversion.Spec)
		if ensureErr == nil {
			if podErr := ensurer.DeletePod(conversion); podErr != nil {
				r.Log.Error(podErr, "Failed to delete pod for canceled Conversion.")
			}
			if _, snapErr := ensurer.RemoveOwnedSnapshot(ctx, conversion); snapErr != nil {
				r.Log.Error(snapErr, "Failed to trigger snapshot removal for canceled Conversion.")
			}
		} else {
			r.Log.Error(ensureErr, "Failed to build Ensurer for canceled Conversion.")
		}
		resolvePhaseConditions(conversion, nil)
		conversion.Status.EndStagingConditions()
		r.Record(conversion, conversion.Status.Conditions)
		conversion.Status.ObservedGeneration = conversion.Generation
		err = r.Status().Update(ctx, conversion)
		result.RequeueAfter = 0
		return
	}

	if conversion.Status.HasBlockerCondition() {
		conversion.Status.EndStagingConditions()
		r.Record(conversion, conversion.Status.Conditions)
		conversion.Status.ObservedGeneration = conversion.Generation
		err = r.Status().Update(ctx, conversion)
		return
	}

	pipe := NewConversionPipeline(ctx, &r, conversion)
	succeeded, pipelineErr := pipe.Run()
	if pipelineErr != nil {
		podSucceeded, podErr := r.conversionPodSucceeded(ctx, conversion)
		if podErr != nil {
			err = podErr
			return
		}
		if podSucceeded {
			r.Log.Info("Conversion pipeline failed but pod succeeded; marking conversion succeeded.",
				"type", conversion.Spec.Type,
				"conversion", conversion.Name)
			pipelineErr = nil
			succeeded = true
		} else {
			r.Log.Error(pipelineErr, "Conversion pipeline failed.",
				"type", conversion.Spec.Type,
				"phase", conversion.Status.Phase,
				"stage", conversion.Status.Stage)
			conversion.Status.Phase = api.PhaseFailed
		}
	}
	if succeeded {
		r.Log.Info("Conversion pipeline succeeded.",
			"type", conversion.Spec.Type)
		conversion.Status.Phase = api.PhaseSucceeded
		conversion.Status.Stage = api.StageFinished
	} else if pipelineErr == nil && conversion.Status.Phase != api.PhaseFailed {
		r.Log.V(3).Info("Conversion pipeline still in progress.",
			"type", conversion.Spec.Type,
			"phase", conversion.Status.Phase,
			"stage", conversion.Status.Stage)
	}

	// Safety net: if the pipeline terminates with Phase=Failed and an owned
	// snapshot is still present (e.g. the failure happened before the divert
	// logic could redirect to StageRemoveSnapshot, or the snapshotRemoval stage
	// itself errored and set Phase=Failed internally), submit a best effort
	// removal task here. If in the primary failure path, the divert in
	// runDeepInspection clears Status.Snapshot before StageFinished (successful snapshot removal) then this block is a no-op.
	if conversion.Status.Phase == api.PhaseFailed {
		ensurer, ensureErr := NewEnsurer(r.Client, r.Log, conversion.Spec)
		if ensureErr == nil {
			if _, snapErr := ensurer.RemoveOwnedSnapshot(ctx, conversion); snapErr != nil {
				r.Log.Error(snapErr, "Failed to trigger snapshot removal for failed Conversion.")
			}
		} else {
			r.Log.Error(ensureErr, "Failed to build Ensurer for snapshot cleanup on failure.")
		}
	}

	resolvePhaseConditions(conversion, pipelineErr)

	conversion.Status.EndStagingConditions()

	r.Record(conversion, conversion.Status.Conditions)

	conversion.Status.ObservedGeneration = conversion.Generation
	err = r.Status().Update(ctx, conversion)
	if err != nil {
		return
	}

	result.RequeueAfter = base.SlowReQ

	return
}

// conversionPod returns the managed conversion pod from status or label lookup.
func (r Reconciler) conversionPod(ctx context.Context, conversion *api.Conversion) (*core.Pod, error) {
	ensurer, err := NewEnsurer(r.Client, r.Log, conversion.Spec)
	if err != nil {
		return nil, err
	}
	if conversion.Status.Pod.Name != "" {
		pod := &core.Pod{}
		err = ensurer.DestinationClient.Get(ctx, types.NamespacedName{
			Namespace: conversion.Status.Pod.Namespace,
			Name:      conversion.Status.Pod.Name,
		}, pod)
		if err == nil {
			return pod, nil
		}
		if !k8serr.IsNotFound(err) {
			return nil, err
		}
	}
	cfg := convctx.PodConfigFromSpec(conversion)
	pod, err := ensurer.GetPod(conversion, cfg.PodLabels)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// conversionPodSucceeded reports whether the managed conversion pod completed successfully.
func (r Reconciler) conversionPodSucceeded(ctx context.Context, conversion *api.Conversion) (bool, error) {
	pod, err := r.conversionPod(ctx, conversion)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	return pod.Status.Phase == core.PodSucceeded, nil
}

// reconcileFailedConversion recovers stale Failed conversions from the managed pod phase.
func (r Reconciler) reconcileFailedConversion(ctx context.Context, conversion *api.Conversion) (recovered bool, requeue bool, err error) {
	pod, err := r.conversionPod(ctx, conversion)
	if err != nil {
		if errors.Is(err, ErrNoPodFound) {
			return false, false, nil
		}
		return false, false, err
	}
	switch pod.Status.Phase {
	case core.PodSucceeded:
		r.markConversionSucceeded(conversion)
		conversion.Status.ObservedGeneration = conversion.Generation
		r.Record(conversion, conversion.Status.Conditions)
		err = r.Status().Update(ctx, conversion)
		r.Log.Info("Recovered failed conversion from succeeded pod.",
			"conversion", conversion.Name,
			"pod", pod.Name)
		return true, false, err
	case core.PodRunning, core.PodPending:
		return false, true, nil
	default:
		return false, false, nil
	}
}

// markConversionSucceeded sets terminal success status and clears stale failure conditions.
func (r Reconciler) markConversionSucceeded(conversion *api.Conversion) {
	conversion.Status.Phase = api.PhaseSucceeded
	conversion.Status.Stage = api.StageFinished
	conversion.Status.DeleteCondition(api.ConversionFailed)
	resolvePhaseConditions(conversion, nil)
}

// resolvePhaseConditions sets the Ready condition on conversion based on the current phase.
// When pipelineErr is non-nil and Phase is Failed, the error detail is included
// in the ConversionFailed condition so the plan controller can surface it.
func resolvePhaseConditions(conversion *api.Conversion, pipelineErr error) {
	switch conversion.Status.Phase {
	case api.PhaseSucceeded:
		now := meta.Now()
		conversion.Status.CompletionTime = &now
		msg := "The conversion has completed successfully."
		conversion.Status.Message = msg
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  msg,
		})
	case api.PhaseFailed:
		now := meta.Now()
		conversion.Status.CompletionTime = &now
		msg := "The conversion has failed."
		if pipelineErr != nil {
			msg = fmt.Sprintf("The conversion has failed: %s", pipelineErr.Error())
		}
		conversion.Status.Message = msg
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     api.ConversionFailed,
			Status:   True,
			Category: Critical,
			Message:  msg,
		})
	case api.PhaseCanceled:
		now := meta.Now()
		conversion.Status.CompletionTime = &now
		msg := "The conversion has been canceled."
		conversion.Status.Message = msg
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     api.ConversionCanceled,
			Status:   True,
			Category: Advisory,
			Message:  msg,
		})
	case api.PhasePending:
		msg := "The conversion is pending."
		conversion.Status.Message = msg
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   False,
			Category: Required,
			Message:  msg,
		})
	case api.PhaseRunning:
		resolveStageConditions(conversion)
	}
}

// resolveStageConditions sets a detailed Advisory message on the conversion
// based on the current pipeline stage.  Called only when Phase == Running.
func resolveStageConditions(conversion *api.Conversion) {
	var msg string
	switch conversion.Status.Stage {
	case api.StageCreatePod:
		msg = "Creating the conversion pod."
	case api.StagePodRunning:
		msg = "Conversion pod is running."
	case api.StageCreateSnapshot:
		msg = "Creating vSphere snapshot."
	case api.StageWaitForSnapshot:
		msg = "Waiting for snapshot creation to complete."
	case api.StageFetchingResults:
		msg = "Fetching inspection results from pod."
	case api.StageRemoveSnapshot:
		msg = "Removing vSphere snapshot."
	case api.StageWaitForSnapshotRemoval:
		msg = "Waiting for snapshot removal to complete."
	case api.StageFinished:
		msg = "Finalizing conversion."
	default:
		msg = "The conversion is running."
	}
	conversion.Status.Message = msg
	conversion.Status.SetCondition(libcnd.Condition{
		Type:     libcnd.Advisory,
		Status:   True,
		Category: Advisory,
		Message:  msg,
	})
}
