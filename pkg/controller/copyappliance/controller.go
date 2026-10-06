package copyappliance

import (
	"context"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add the controller to the manager and watch CopyAppliance resources.
func Add(mgr manager.Manager) error {
	reconciler := &Reconciler{
		Reconciler: base.Reconciler{
			Client:        mgr.GetClient(),
			EventRecorder: mgr.GetEventRecorderFor(Name),
			Log:           log,
		},
	}
	cnt, err := controller.New(
		Name,
		mgr,
		controller.Options{
			MaxConcurrentReconciles: Settings.MaxConcurrentReconciles,
			Reconciler:              reconciler,
		})
	if err != nil {
		log.Trace(err)
		return err
	}
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.CopyAppliance{},
			&handler.TypedEnqueueRequestForObject[*api.CopyAppliance]{},
		))
	if err != nil {
		log.Trace(err)
		return err
	}
	// The appliance is set up by a pod, so the pod's progress is what the
	// PhaseSetupAppliance step is waiting on.
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &core.Pod{}, applianceForSetupPodMapper(),
			predicate.NewTypedPredicateFuncs(func(obj *core.Pod) bool {
				_, ok := obj.Labels[api.LabelCopyApplianceSetup]
				return ok
			})))
	if err != nil {
		log.Trace(err)
		return err
	}
	return nil
}

// applianceForSetupPodMapper maps a setup pod to the CopyAppliance it was
// created for, which is in the pod's own namespace.
func applianceForSetupPodMapper() handler.TypedEventHandler[*core.Pod, reconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(_ context.Context, pod *core.Pod) []reconcile.Request {
		name := pod.Labels[api.LabelCopyApplianceSetup]
		if name == "" {
			return nil
		}
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      name,
			},
		}}
	})
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciles a CopyAppliance object.
type Reconciler struct {
	base.Reconciler
}

// Reconcile a CopyAppliance CR.
// Note: Must not a pointer receiver to ensure that the
// logger and other state is not shared.
func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"copy-appliance",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	appliance := &api.CopyAppliance{}
	err = r.Get(ctx, request.NamespacedName, appliance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("Resource deleted.")
			err = nil
		}
		return
	}

	defer func() {
		r.Log.V(2).Info("Conditions.", "all", appliance.Status.Conditions)
	}()

	deleting := !appliance.DeletionTimestamp.IsZero()
	terminalPhase := appliance.Status.Phase == api.PhaseDeployCompleted || appliance.Status.Phase == api.PhaseReleased
	if !deleting && terminalPhase && !PendingExportRequest(appliance) {
		r.Log.Info("Nothing to do.")
		return
	}

	var reQ time.Duration
	appliance.Status.BeginStagingConditions()
	if deleting {
		reQ, err = r.Teardown(ctx, appliance)
	} else {
		patch := client.MergeFrom(appliance.DeepCopy())
		if controllerutil.AddFinalizer(appliance, api.CopyApplianceFinalizer) {
			err = r.Patch(ctx, appliance, patch)
			if err != nil {
				err = liberr.Wrap(err)
				r.Log.Error(err, "failed to add finalizer", "appliance", appliance.Name, "namespace", appliance.Namespace)
				return
			}
			// Return after the finalizer patch: continuing would Deploy against a
			// stale resourceVersion, Status().Update would conflict, and the next
			// reconcile would restart CloneVM (double CloneVM_Task).
			return
		}
		switch appliance.Status.Phase {
		case api.PhaseReleaseDisks, api.PhaseWaitForReleaseDisks,
			api.PhaseAttachDisks, api.PhaseWaitForAttachDisks,
			api.PhaseRestartOrchestrator,
			api.PhaseDeployCompleted, api.PhaseReleased:
			reQ, err = r.Export(ctx, appliance)
		case api.PhaseWaitForExports:
			// Shared with deploy; AttachDisks means export owns this wait.
			if wantExport(appliance) {
				reQ, err = r.Export(ctx, appliance)
			} else {
				reQ, err = r.Deploy(ctx, appliance)
			}
		default:
			reQ, err = r.Deploy(ctx, appliance)
		}
	}
	appliance.Status.EndStagingConditions()

	// The status is written even when the pass failed. The runner advanced the
	// phase before it returned, and dropping that means repeating the vSphere
	// work the pass did manage to do.
	r.Record(appliance, appliance.Status.Conditions)
	uErr := r.Status().Update(ctx, appliance)
	if uErr != nil {
		r.Log.Error(uErr, "Failed to update status.")
		if err == nil {
			err = liberr.Wrap(uErr)
		}
	}

	result.RequeueAfter = reQ

	// Released after the status update, because releasing it lets the API
	// server delete the object out from under us.
	if deleting {
		fErr := r.RemoveFinalizer(ctx, appliance)
		if fErr != nil {
			r.Log.Error(fErr, "Failed to remove finalizer.")
			if err == nil {
				err = liberr.Wrap(fErr)
			}
		}
	}
	return
}

// RemoveFinalizer releases the finalizer once the appliance VM is gone. It must
// not be released sooner: an appliance left behind holds read locks on the
// source vmdks with nothing left in the cluster to point at it.
func (r *Reconciler) RemoveFinalizer(ctx context.Context, appliance *api.CopyAppliance) (err error) {
	if appliance.Status.Phase != api.PhaseTeardownCompleted {
		return
	}
	patch := client.MergeFrom(appliance.DeepCopy())
	if controllerutil.RemoveFinalizer(appliance, api.CopyApplianceFinalizer) {
		err = r.Patch(ctx, appliance, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to remove finalizer", "appliance", appliance.Name, "namespace", appliance.Namespace)
			return
		}
	}
	return
}

// ApplianceContext resolves the referenced provider and its secret and connects
// to the source provider. The caller owns the returned context and must Close
// it.
func (r *Reconciler) ApplianceContext(ctx context.Context, appliance *api.CopyAppliance) (ac *ApplianceContext, err error) {
	providerKey := types.NamespacedName{
		Namespace: appliance.Spec.Provider.Namespace,
		Name:      appliance.Spec.Provider.Name,
	}
	provider := &api.Provider{}
	err = r.Get(ctx, providerKey, provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	secretKey := types.NamespacedName{
		Namespace: provider.Spec.Secret.Namespace,
		Name:      provider.Spec.Secret.Name,
	}
	secret := &core.Secret{}
	err = r.Get(ctx, secretKey, secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	applianceSecret, err := r.applianceSecret(ctx, appliance)
	if err != nil {
		return
	}
	ac, err = NewApplianceContext(ctx, appliance, provider, secret, applianceSecret, r.Log)
	if err != nil {
		return
	}
	return
}

// applianceSecret resolves the secret the appliance is reached with: the SSH
// key pair it is configured over and the mutual-TLS material it serves its
// exports with. Spec.Secret.Name is required. A named secret that is not there
// is reported as none rather than as an error: failing here would stop a
// teardown too, leaving an appliance that cannot be deleted and read locks on
// the source vmdks with nothing left to release them. Only the configure and
// export steps need the object, and they name the secret when it is missing.
func (r *Reconciler) applianceSecret(ctx context.Context, appliance *api.CopyAppliance) (secret *core.Secret, err error) {
	ref := appliance.Spec.Secret
	if ref.Name == "" {
		err = liberr.New("the appliance secret is required")
		return
	}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = appliance.Namespace
	}
	key := types.NamespacedName{Namespace: namespace, Name: ref.Name}
	found := &core.Secret{}
	err = r.Get(ctx, key, found)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("The appliance secret is not there.",
				"namespace", key.Namespace,
				"name", key.Name)
			err = nil
			return
		}
		err = liberr.Wrap(err)
		return
	}
	secret = found
	return
}

// Deploy drives the appliance VM one step closer to running and returns. Each
// step that has to wait on vSphere leaves a phase behind and names the interval
// it wants to be polled at, rather than blocking the worker on a poll loop.
// A pass that never reached the runner has no interval to report, so the
// backoff for a failed connection is chosen here.
func (r *Reconciler) Deploy(ctx context.Context, appliance *api.CopyAppliance) (reQ time.Duration, err error) {
	applianceContext, err := r.ApplianceContext(ctx, appliance)
	if err != nil {
		r.setFailed(appliance, api.PhaseDeployFailed, "ConnectFailed", err)
		reQ = base.LongReQ
		err = nil
		return
	}
	defer applianceContext.Close()

	r.forgetForeignVM(appliance, applianceContext.InstanceUUID())

	runner := DeployRunner{context: applianceContext, client: r.Client}
	reQ, err = runner.Run(ctx)
	if err != nil {
		r.setFailed(appliance, api.PhaseDeployFailed, "DeployFailed", err)
		err = nil
		return
	}
	r.setConverging(appliance, "The copy appliance is being deployed.")
	return
}

// Export drives disk release or re-export on a deployed appliance.
func (r *Reconciler) Export(ctx context.Context, appliance *api.CopyAppliance) (reQ time.Duration, err error) {
	applianceContext, err := r.ApplianceContext(ctx, appliance)
	if err != nil {
		r.setFailed(appliance, api.PhaseDeployFailed, "ConnectFailed", err)
		reQ = base.LongReQ
		err = nil
		return
	}
	defer applianceContext.Close()

	r.forgetForeignVM(appliance, applianceContext.InstanceUUID())

	runner := ExportRunner{context: applianceContext}
	reQ, err = runner.Run(ctx)
	if err != nil {
		r.setFailed(appliance, api.PhaseDeployFailed, "ExportFailed", err)
		err = nil
		return
	}
	r.setConverging(appliance, "The copy appliance export is being updated.")
	return
}

// Teardown drives the appliance VM one step closer to gone and returns. Each
// step that has to wait on vSphere leaves a phase behind and names the interval
// it wants to be polled at, rather than blocking the worker on a poll loop.
func (r *Reconciler) Teardown(ctx context.Context, appliance *api.CopyAppliance) (reQ time.Duration, err error) {
	applianceContext, err := r.ApplianceContext(ctx, appliance)
	if err != nil {
		r.setFailed(appliance, api.PhaseTeardownFailed, "ConnectFailed", err)
		reQ = base.LongReQ
		err = nil
		return
	}
	defer applianceContext.Close()

	r.forgetForeignVM(appliance, applianceContext.InstanceUUID())

	runner := TeardownRunner{context: applianceContext, client: r.Client}
	reQ, err = runner.Run(ctx)
	if err != nil {
		r.setFailed(appliance, api.PhaseTeardownFailed, "TeardownFailed", err)
		err = nil
		return
	}
	r.setConverging(appliance, "The copy appliance is being torn down.")
	return
}

// forgetForeignVM discards a recorded moRef that was written against a
// different vCenter than the one we are connected to. Acting on it would mean
// powering on — or destroying — an unrelated VM. Status fields that describe a
// specific VM are only meaningful together, so they are always cleared
// together; clearing the phase restarts the itinerary on the next pass.
func (r *Reconciler) forgetForeignVM(appliance *api.CopyAppliance, instanceUUID string) {
	recorded := appliance.Status.VCenterInstanceUUID
	if appliance.Status.MoRef == "" || recorded == "" || instanceUUID == "" || recorded == instanceUUID {
		return
	}
	r.Log.Info("Recorded appliance VM belongs to a different vCenter; ignoring it.",
		"vm", appliance.Status.MoRef,
		"recorded", recorded,
		"connected", instanceUUID)
	appliance.Status.MoRef = ""
	appliance.Status.Addresses = nil
	appliance.Status.Exports = nil
	appliance.Status.VCenterInstanceUUID = ""
	appliance.Status.TaskRef = ""
	appliance.Status.Phase = ""
}

// setFailed records a failed pass as a not-ready condition and a failed phase.
// The phase is passed in because a failure during teardown must not be recorded
// as a deployment failure: they requeue the same way but read very differently.
// Durable so staging on the next reconcile does not drop the root-cause message
// before PhaseDeployFailed / the plan can read it.
func (r *Reconciler) setFailed(appliance *api.CopyAppliance, phase, reason string, err error) {
	appliance.Status.Phase = phase
	appliance.Status.SetCondition(libcnd.Condition{
		Type:     libcnd.Ready,
		Status:   libcnd.False,
		Reason:   reason,
		Category: libcnd.Error,
		Message:  err.Error(),
		Durable:  true,
	})
}

// FailureReason returns the message the appliance recorded when it failed. The
// category is checked because setConverging writes a Ready condition too, on
// every non-terminal phase, and that one is not a failure. setFailed writes
// Error; PhaseDeployFailed / PhaseTeardownFailed promote it to Critical — both
// carry the root cause.
func FailureReason(appliance *api.CopyAppliance) string {
	cnd := appliance.Status.FindCondition(libcnd.Ready)
	if cnd != nil && cnd.Message != "" &&
		(cnd.Category == libcnd.Error || cnd.Category == libcnd.Critical) {
		return cnd.Message
	}
	return "the appliance did not record why it failed"
}

// setConverging records that the appliance is still on its way to the phase it
// is headed for. A terminal phase has already set its own condition, and
// staging would otherwise leave the appliance with no Ready condition at all
// between passes. This is not an error, so it is Advisory.
func (r *Reconciler) setConverging(appliance *api.CopyAppliance, message string) {
	phase := appliance.Status.Phase
	switch phase {
	case api.PhaseDeployCompleted, api.PhaseDeployFailed,
		api.PhaseReleased,
		api.PhaseTeardownCompleted, api.PhaseTeardownFailed:
		return
	}
	appliance.Status.SetCondition(libcnd.Condition{
		Type:     libcnd.Ready,
		Status:   libcnd.False,
		Reason:   phase,
		Category: libcnd.Advisory,
		Message:  message,
	})
}
