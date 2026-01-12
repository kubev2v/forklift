package hyperv

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensurer ensures resources exist.
type Ensurer struct {
	client.Client
	Log                  logging.LevelLogger
	HyperVProviderServer *api.HyperVProviderServer
	Labeler              Labeler
}

// ProviderServer ensures the HyperVProviderServer resource exists.
// Uses label selector to find existing servers (supports GenerateName).
func (r *Ensurer) ProviderServer(ctx context.Context, server *api.HyperVProviderServer) (out *api.HyperVProviderServer, err error) {
	list := &api.HyperVProviderServerList{}
	err = r.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(server.Labels),
		Namespace:     server.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var existing []api.HyperVProviderServer
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
		r.Log.Info("Created HyperVProviderServer.", "server", server.Name, "namespace", server.Namespace)
		out = server
	} else {
		out = &existing[0]
	}
	return
}

// PersistentVolume ensures the static PV exists.
// Uses label selector to find existing PVs (supports GenerateName).
func (r *Ensurer) PersistentVolume(ctx context.Context, pv *core.PersistentVolume) (out *core.PersistentVolume, err error) {
	list := &core.PersistentVolumeList{}
	err = r.List(ctx, list, &client.ListOptions{
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
		r.Log.Info("Created PersistentVolume.", "pv", pv.Name)
		out = pv
	} else {
		out = &list.Items[0]
	}
	return
}

// PersistentVolumeClaim ensures the PVC exists.
// Uses label selector to find existing PVCs (supports GenerateName).
func (r *Ensurer) PersistentVolumeClaim(ctx context.Context, pvc *core.PersistentVolumeClaim) (out *core.PersistentVolumeClaim, err error) {
	list := &core.PersistentVolumeClaimList{}
	err = r.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(pvc.Labels),
		Namespace:     pvc.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		// Set owner reference so that GC can clean up if the server is deleted.
		if r.HyperVProviderServer != nil {
			err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.HyperVProviderServer, pvc)
			if err != nil {
				return
			}
		}
		err = r.Create(ctx, pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created PersistentVolumeClaim.", "pvc", pvc.Name, "namespace", pvc.Namespace)
		out = pvc
	} else {
		out = &list.Items[0]
	}
	return
}

// Deployment ensures the Deployment exists.
func (r *Ensurer) Deployment(ctx context.Context, deployment *appsv1.Deployment) (err error) {
	list := &appsv1.DeploymentList{}
	err = r.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(deployment.Labels),
		Namespace:     deployment.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		// Set owner reference on namespaced Deployment to enable GC.
		if r.HyperVProviderServer != nil {
			err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.HyperVProviderServer, deployment)
			if err != nil {
				return
			}
		}
		err = r.Create(ctx, deployment)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created Deployment.", "deployment", deployment.Name, "namespace", deployment.Namespace)
	}
	return
}

// Service ensures the Service exists.
func (r *Ensurer) Service(ctx context.Context, service *core.Service) (out *core.Service, err error) {
	list := &core.ServiceList{}
	err = r.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(service.Labels),
		Namespace:     service.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		// Set owner reference so the Service is GC'd with the server.
		if r.HyperVProviderServer != nil {
			err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.HyperVProviderServer, service)
			if err != nil {
				return
			}
		}
		err = r.Create(ctx, service)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created Service.", "service", service.Name, "namespace", service.Namespace)
		out = service
	} else {
		out = &list.Items[0]
	}
	return
}
