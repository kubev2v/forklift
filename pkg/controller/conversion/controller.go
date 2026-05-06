package conversion

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

	return nil
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

	// only reconcile if the conversion pod is not finished
	if conversion.Status.Phase == api.PhaseSucceeded || conversion.Status.Phase == api.PhaseFailed {
		result.RequeueAfter = 0
		err = nil
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
		resolvePhaseConditions(conversion)
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
	succeeded, err := pipe.Run()
	if err != nil {
		r.Log.Error(err, "Conversion pipeline failed.",
			"type", conversion.Spec.Type,
			"phase", conversion.Status.Phase,
			"stage", conversion.Status.Stage)
		conversion.Status.Phase = api.PhaseFailed
	} else if succeeded {
		r.Log.Info("Conversion pipeline succeeded.",
			"type", conversion.Spec.Type)
		conversion.Status.Phase = api.PhaseSucceeded
	} else {
		r.Log.V(3).Info("Conversion pipeline still in progress.",
			"type", conversion.Spec.Type,
			"phase", conversion.Status.Phase,
			"stage", conversion.Status.Stage)
	}

	resolvePhaseConditions(conversion)

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

// resolvePhaseConditions sets the Ready condition on conversion based on the current phase.
func resolvePhaseConditions(conversion *api.Conversion) {
	switch conversion.Status.Phase {
	case api.PhaseSucceeded:
		now := meta.Now()
		conversion.Status.CompletionTime = &now
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The conversion has completed successfully.",
		})
	case api.PhaseFailed:
		now := meta.Now()
		conversion.Status.CompletionTime = &now
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     "ConversionFailed",
			Status:   True,
			Category: Critical,
			Message:  "The conversion has failed.",
		})
	case api.PhaseCanceled:
		now := meta.Now()
		conversion.Status.CompletionTime = &now
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     "ConversionCanceled",
			Status:   True,
			Category: Advisory,
			Message:  "The conversion has been canceled.",
		})
	case api.PhasePending:
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   False,
			Category: Required,
			Message:  "The conversion is pending.",
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
	conversion.Status.SetCondition(libcnd.Condition{
		Type:     libcnd.Advisory,
		Status:   True,
		Category: Advisory,
		Message:  msg,
	})
}
