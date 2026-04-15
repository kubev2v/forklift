package conversion

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
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
			Reconciler: reconciler,
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

	if conversion.Status.Phase == api.PhaseSucceeded || conversion.Status.Phase == api.PhaseFailed {
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

	if conversion.Status.HasBlockerCondition() {
		conversion.Status.EndStagingConditions()
		r.Record(conversion, conversion.Status.Conditions)
		conversion.Status.ObservedGeneration = conversion.Generation
		err = r.Status().Update(ctx, conversion)
		return
	}

	conversion.Status.Phase = api.PhaseCreating

	// Ensure the virt-v2v pod exists and track its state.
	err = r.ensurePod(ctx, conversion)
	if err != nil {
		return
	}

	// Set phase and Ready condition based on pod state.
	switch conversion.Status.Phase {
	case api.PhaseSucceeded:
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   True,
			Category: Required,
			Message:  "The conversion has completed successfully.",
		})
	case api.PhaseFailed:
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     "PodFailed",
			Status:   True,
			Category: Critical,
			Message:  "The conversion pod has failed.",
		})
	default:
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   False,
			Category: Required,
			Message:  "The conversion pod is running.",
		})
	}

	conversion.Status.EndStagingConditions()

	r.Record(conversion, conversion.Status.Conditions)

	conversion.Status.ObservedGeneration = conversion.Generation
	err = r.Status().Update(ctx, conversion)
	if err != nil {
		return
	}
	if conversion.Status.Phase != api.PhaseSucceeded && conversion.Status.Phase != api.PhaseFailed {
		result.RequeueAfter = base.SlowReQ
	}
	return
}
