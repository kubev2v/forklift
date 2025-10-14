package ova

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name = "ova-server"
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
		if kerrors.IsNotFound(err) {
			r.Log.Info("OVA provider server deleted.")
			err = nil
		}
		return
	}

	defer func() {
		r.Log.V(2).Info("Conditions.", "all", ova.Status.Conditions)
	}()

	err = r.DeployOVAProviderServer(ctx, ova)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	err = r.Status().Update(ctx, ova)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Done.
	return
}

func (r *Reconciler) DeployOVAProviderServer(ctx context.Context, ova *api.OVAProviderServer) (err error) {
	if ova.Status.Phase == libcnd.Ready {
		return
	}
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

	builder := Builder{OVAProviderServer: ova}
	ensurer := Ensurer{
		Client: r.Client,
	}
	pv := builder.PersistentVolume(provider)
	pv, err = ensurer.PersistentVolume(ctx, pv)
	if err != nil {
		return
	}
	pvc := builder.PersistentVolumeClaim(provider, pv)
	pvc, err = ensurer.PersistentVolumeClaim(ctx, pvc)
	if err != nil {
		return
	}
	deployment := builder.Deployment(provider, pvc)
	err = ensurer.Deployment(ctx, deployment)
	if err != nil {
		return
	}
	service := builder.Service(provider)
	service, err = ensurer.Service(ctx, service)
	if err != nil {
		return
	}
	ova.Status.Service = &v1.ObjectReference{
		Kind:      "Service",
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	ova.Status.Phase = libcnd.Ready
	return
}
