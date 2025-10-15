package ova

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Ensurer struct {
	OVAProviderServer *api.OVAProviderServer
	Labeler           Labeler
	Log               logging.LevelLogger
	k8sclient.Client
}

func (r *Ensurer) ProviderServer(ctx context.Context, server *api.OVAProviderServer) (out *api.OVAProviderServer, err error) {
	list := &api.OVAProviderServerList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(server.Labels),
		Namespace:     server.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var existing []api.OVAProviderServer
	for i := range list.Items {
		item := list.Items[i]
		if item.DeletionTimestamp != nil {
			continue
		} else {
			existing = append(existing, item)
		}
	}
	if len(existing) == 0 {
		err = r.Create(ctx, server)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created OVAProviderServer.", "server", server.Name, "namespace", server.Namespace)
		out = server
	} else {
		out = &existing[0]
	}
	return
}

func (r *Ensurer) PersistentVolume(ctx context.Context, pv *core.PersistentVolume) (out *core.PersistentVolume, err error) {
	list := &core.PersistentVolumeList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(pv.Labels),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		err = r.Create(ctx, pv)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created PersistentVolume.", "pv", pv.Name, "server", r.OVAProviderServer.Name, "namespace", r.OVAProviderServer.Namespace)
		out = pv
	} else {
		out = &list.Items[0]
	}
	return
}

func (r *Ensurer) PersistentVolumeClaim(ctx context.Context, pvc *core.PersistentVolumeClaim) (out *core.PersistentVolumeClaim, err error) {
	list := &core.PersistentVolumeClaimList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(pvc.Labels),
		Namespace:     pvc.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.OVAProviderServer, pvc)
		if err != nil {
			return
		}
		err = r.Create(ctx, pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created PersistentVolumeClaim.", "pvc", pvc.Name, "server", r.OVAProviderServer.Name, "namespace", r.OVAProviderServer.Namespace)
		out = pvc
	} else {
		out = &list.Items[0]
	}
	return
}

func (r *Ensurer) Deployment(ctx context.Context, deployment *appsv1.Deployment) (err error) {
	list := &appsv1.DeploymentList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(deployment.Labels),
		Namespace:     deployment.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.OVAProviderServer, deployment)
		if err != nil {
			return
		}
		err = r.Create(ctx, deployment)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created Deployment.", "deployment", deployment.Name, "server", r.OVAProviderServer.Name, "namespace", r.OVAProviderServer.Namespace)
	}
	return
}

func (r *Ensurer) Service(ctx context.Context, svc *core.Service) (out *core.Service, err error) {
	list := &core.ServiceList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(svc.Labels),
		Namespace:     svc.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.OVAProviderServer, svc)
		if err != nil {
			return
		}
		err = r.Create(ctx, svc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created Service.", "service", svc.Name, "server", r.OVAProviderServer.Name, "namespace", r.OVAProviderServer.Namespace)
		out = svc
	} else {
		out = &list.Items[0]
	}
	return
}
