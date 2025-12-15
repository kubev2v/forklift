package hyperv

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Deleter deletes resources.
type Deleter struct {
	HyperVProviderServer *api.HyperVProviderServer
	Labeler              Labeler
	Log                  logging.LevelLogger
	k8sclient.Client
}

// Service deletes the Service.
func (r *Deleter) Service(ctx context.Context, provider *api.Provider) (err error) {
	list := &core.ServiceList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(r.Labeler.ServerLabels(provider, r.HyperVProviderServer)),
		Namespace:     r.HyperVProviderServer.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		item := &list.Items[i]
		err = r.Delete(ctx, item)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = nil
				continue
			}
			r.Log.Error(err, "failed to delete Service for provider server", "service", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
			return
		}
		r.Log.Info("deleted Service for provider server", "service", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
	}
	return
}

// Deployment deletes the Deployment.
func (r *Deleter) Deployment(ctx context.Context, provider *api.Provider) (err error) {
	list := &appsv1.DeploymentList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(r.Labeler.ServerLabels(provider, r.HyperVProviderServer)),
		Namespace:     r.HyperVProviderServer.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		item := &list.Items[i]
		err = r.Delete(ctx, item)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = nil
				continue
			}
			r.Log.Error(err, "failed to delete Deployment for provider server", "deployment", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
			return
		}
		r.Log.Info("deleted Deployment for provider server", "deployment", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
	}
	return
}

// PersistentVolumeClaim deletes the PVC.
func (r *Deleter) PersistentVolumeClaim(ctx context.Context, provider *api.Provider) (err error) {
	list := &core.PersistentVolumeClaimList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(r.Labeler.ServerLabels(provider, r.HyperVProviderServer)),
		Namespace:     r.HyperVProviderServer.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		item := &list.Items[i]
		err = r.Delete(ctx, item)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = nil
				continue
			}
			r.Log.Error(err, "failed to delete PersistentVolumeClaim for provider server", "pvc", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
			return
		}
		r.Log.Info("deleted PersistentVolumeClaim for provider server", "pvc", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
	}
	return
}

// PersistentVolume deletes the static PV.
func (r *Deleter) PersistentVolume(ctx context.Context, provider *api.Provider) (err error) {
	list := &core.PersistentVolumeList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(r.Labeler.ServerLabels(provider, r.HyperVProviderServer)),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range list.Items {
		item := &list.Items[i]
		err = r.Delete(ctx, item)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = nil
				continue
			}
			r.Log.Error(err, "failed to delete PersistentVolume for provider server", "pv", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
			return
		}
		r.Log.Info("deleted PersistentVolume for provider server", "pv", item.Name, "server", r.HyperVProviderServer.Name, "namespace", r.HyperVProviderServer.Namespace)
	}
	return
}
