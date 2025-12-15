package hyperv

import (
	"context"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name           = "hyperv-server"
	FeatureEnabled = "FeatureEnabled"
)

var log = logging.WithName(Name)

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
	// Primary CR.
	err = cnt.Watch(
		source.Kind(mgr.GetCache(), &api.HyperVProviderServer{},
			&handler.TypedEnqueueRequestForObject[*api.HyperVProviderServer]{},
		))
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

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	r.Log = logging.WithName(
		names.SimpleNameGenerator.GenerateName(Name+"|"),
		"provider",
		request)
	r.Started()
	defer func() {
		result.RequeueAfter = r.Ended(
			result.RequeueAfter,
			err)
		err = nil
	}()

	hyperv := &api.HyperVProviderServer{}
	err = r.Get(ctx, request.NamespacedName, hyperv)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("HyperV provider server deleted.")
			err = nil
		}
		return
	}

	defer func() {
		r.Log.V(2).Info("Conditions.", "all", hyperv.Status.Conditions)
	}()

	if hyperv.DeletionTimestamp.IsZero() {
		err = r.AddFinalizer(ctx, hyperv)
		if err != nil {
			return
		}
		err = r.Deploy(ctx, hyperv)
		if err != nil {
			return
		}
	} else {
		err = r.Teardown(ctx, hyperv)
		if err != nil {
			return
		}
		err = r.RemoveFinalizer(ctx, hyperv)
		if err != nil {
			return
		}
	}
	err = r.Status().Update(ctx, hyperv)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

func (r *Reconciler) AddFinalizer(ctx context.Context, hyperv *api.HyperVProviderServer) (err error) {
	patch := client.MergeFrom(hyperv.DeepCopy())
	if controllerutil.AddFinalizer(hyperv, api.HyperVProviderFinalizer) {
		err = r.Patch(ctx, hyperv, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to add finalizer", "server", hyperv.Name, "namespace", hyperv.Namespace)
			return
		}
	}
	return
}

func (r *Reconciler) RemoveFinalizer(ctx context.Context, hyperv *api.HyperVProviderServer) (err error) {
	patch := client.MergeFrom(hyperv.DeepCopy())
	if controllerutil.RemoveFinalizer(hyperv, api.HyperVProviderFinalizer) {
		err = r.Patch(ctx, hyperv, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to remove finalizer", "server", hyperv.Name, "namespace", hyperv.Namespace)
			return
		}
	}
	return
}

// Deploy creates the HyperV provider server deployment.
func (r *Reconciler) Deploy(ctx context.Context, hyperv *api.HyperVProviderServer) (err error) {
	provider := &api.Provider{}
	err = r.Get(
		ctx,
		types.NamespacedName{
			Namespace: hyperv.Spec.Provider.Namespace,
			Name:      hyperv.Spec.Provider.Name,
		},
		provider,
	)
	if err != nil {
		log.Error(err, "Failed to get provider CR.")
		err = liberr.Wrap(err)
		return
	}

	// Get the provider secret
	secret := &v1.Secret{}
	err = r.Get(
		ctx,
		types.NamespacedName{
			Namespace: provider.Spec.Secret.Namespace,
			Name:      provider.Spec.Secret.Name,
		},
		secret,
	)
	if err != nil {
		log.Error(err, "Failed to get provider secret.")
		err = liberr.Wrap(err)
		return
	}

	build := Builder{HyperVProviderServer: hyperv}
	ensure := Ensurer{
		Client:               r.Client,
		Log:                  r.Log,
		HyperVProviderServer: hyperv,
		Labeler:              Labeler{},
	}

	// Create static PV for SMB CSI driver
	pv := build.PersistentVolume(provider, secret)
	pv, err = ensure.PersistentVolume(ctx, pv)
	if err != nil {
		return
	}

	// Create PVC bound to the static PV
	pvc := build.PersistentVolumeClaim(provider, pv)
	pvc, err = ensure.PersistentVolumeClaim(ctx, pvc)
	if err != nil {
		return
	}

	deployment := build.Deployment(provider, pvc)
	err = ensure.Deployment(ctx, deployment)
	if err != nil {
		return
	}

	// Create service for inventory access
	service := build.Service(provider)
	service, err = ensure.Service(ctx, service)
	if err != nil {
		return
	}

	if r.managementEndpoints(deployment) {
		hyperv.Status.SetCondition(
			libcnd.Condition{
				Type:     ApplianceManagementEnabled,
				Status:   libcnd.True,
				Reason:   FeatureEnabled,
				Category: libcnd.Advisory,
				Message:  "HyperV appliance management endpoints are enabled for this provider.",
			})
	}
	hyperv.Status.Service = &v1.ObjectReference{
		Kind:      "Service",
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	hyperv.Status.Phase = libcnd.Ready
	return
}

// Teardown deletes the HyperV provider server resources.
// SMB CSI will automatically delete the PV when the PVC is deleted.
func (r *Reconciler) Teardown(ctx context.Context, hyperv *api.HyperVProviderServer) (err error) {
	provider := &api.Provider{}
	err = r.Get(
		ctx,
		types.NamespacedName{
			Namespace: hyperv.Spec.Provider.Namespace,
			Name:      hyperv.Spec.Provider.Name,
		},
		provider,
	)
	if err != nil {
		// If the Provider CR no longer exists, proceed with best-effort teardown using labels
		// carried by the HyperVProviderServer. This allows finalizer removal and prevents
		// orphaned Deployments/Services/PVCs from sticking around.
		if k8serrors.IsNotFound(err) {
			// Synthesize a minimal provider carrying the UID and names used in labels/selectors.
			provider = &api.Provider{}
			provider.Namespace = hyperv.Spec.Provider.Namespace
			provider.Name = hyperv.Spec.Provider.Name
			if uid, ok := hyperv.Labels[LabelProvider]; ok {
				provider.UID = types.UID(uid)
			}
			// Do not return an error; continue with deletion using synthesized provider.
		} else {
			log.Error(err, "Failed to get provider CR.")
			err = liberr.Wrap(err)
			return
		}
	}

	del := Deleter{
		HyperVProviderServer: hyperv,
		Client:               r.Client,
		Log:                  r.Log,
	}

	err = del.Service(ctx, provider)
	if err != nil {
		return
	}
	err = del.Deployment(ctx, provider)
	if err != nil {
		return
	}
	// Delete PVC
	err = del.PersistentVolumeClaim(ctx, provider)
	if err != nil {
		return
	}
	// Delete static PV
	err = del.PersistentVolume(ctx, provider)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) managementEndpoints(deployment *appsv1.Deployment) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == ApplianceEndpoints {
				return env.Value == strconv.FormatBool(true)
			}
		}
	}
	return false
}
