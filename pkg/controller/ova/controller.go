package ova

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
	Name                       = "ova-server"
	ApplianceManagementEnabled = "ApplianceManagementEnabled"
	FeatureEnabled             = "FeatureEnabled"
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
		source.Kind(mgr.GetCache(), &api.OVAProviderServer{},
			&handler.TypedEnqueueRequestForObject[*api.OVAProviderServer]{},
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

	ova := &api.OVAProviderServer{}
	err = r.Get(ctx, request.NamespacedName, ova)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("OVA provider server deleted.")
			err = nil
		}
		return
	}

	defer func() {
		r.Log.V(2).Info("Conditions.", "all", ova.Status.Conditions)
	}()

	if ova.DeletionTimestamp.IsZero() {
		err = r.AddFinalizer(ctx, ova)
		if err != nil {
			return
		}
		err = r.Deploy(ctx, ova)
		if err != nil {
			return
		}
	} else {
		err = r.Teardown(ctx, ova)
		if err != nil {
			return
		}
		err = r.RemoveFinalizer(ctx, ova)
		if err != nil {
			return
		}
	}
	err = r.Status().Update(ctx, ova)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Done.
	return
}

func (r *Reconciler) AddFinalizer(ctx context.Context, ova *api.OVAProviderServer) (err error) {
	patch := client.MergeFrom(ova.DeepCopy())
	if controllerutil.AddFinalizer(ova, api.OvaProviderFinalizer) {
		err = r.Patch(ctx, ova, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to add finalizer", "server", ova.Name, "namespace", ova.Namespace)
			return
		}
	}
	return
}

func (r *Reconciler) RemoveFinalizer(ctx context.Context, ova *api.OVAProviderServer) (err error) {
	patch := client.MergeFrom(ova.DeepCopy())
	if controllerutil.RemoveFinalizer(ova, api.OvaProviderFinalizer) {
		err = r.Patch(ctx, ova, patch)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "failed to remove finalizer", "server", ova.Name, "namespace", ova.Namespace)
			return
		}
	}
	return
}
func (r *Reconciler) Deploy(ctx context.Context, ova *api.OVAProviderServer) (err error) {
	provider := &api.Provider{}
	err = r.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ova.Spec.Provider.Namespace,
			Name:      ova.Spec.Provider.Name,
		},
		provider,
	)
	if err != nil {
		log.Error(err, "Failed to get provider CR.")
		err = liberr.Wrap(err)
		return
	}

	build := Builder{OVAProviderServer: ova}
	ensure := Ensurer{
		Client:            r.Client,
		OVAProviderServer: ova,
		Log:               r.Log,
	}
	pv := build.PersistentVolume(provider)
	pv, err = ensure.PersistentVolume(ctx, pv)
	if err != nil {
		return
	}
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
	service := build.Service(provider)
	service, err = ensure.Service(ctx, service)
	if err != nil {
		return
	}
	if r.managementEndpoints(deployment) {
		ova.Status.SetCondition(
			libcnd.Condition{
				Type:     ApplianceManagementEnabled,
				Status:   libcnd.True,
				Reason:   FeatureEnabled,
				Category: libcnd.Advisory,
				Message:  "OVA appliance management endpoints are enabled for this provider.",
			})
	}
	ova.Status.Service = &v1.ObjectReference{
		Kind:      "Service",
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	ova.Status.Phase = libcnd.Ready
	return
}

func (r *Reconciler) Teardown(ctx context.Context, ova *api.OVAProviderServer) (err error) {
	provider := &api.Provider{}
	err = r.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ova.Spec.Provider.Namespace,
			Name:      ova.Spec.Provider.Name,
		},
		provider,
	)
	if err != nil {
		log.Error(err, "Failed to get provider CR.")
		err = liberr.Wrap(err)
		return
	}
	del := Deleter{
		OVAProviderServer: ova,
		Client:            r.Client,
		Log:               r.Log,
	}
	err = del.Service(ctx, provider)
	if err != nil {
		return
	}
	err = del.Deployment(ctx, provider)
	if err != nil {
		return
	}
	err = del.PersistentVolumeClaim(ctx, provider)
	if err != nil {
		return
	}
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
