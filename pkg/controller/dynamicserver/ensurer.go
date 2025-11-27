package dynamicserver

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
	Server  *api.DynamicProviderServer
	Labeler Labeler
	Log     logging.LevelLogger
	k8sclient.Client
}

func (r *Ensurer) FindProviderServerByProvider(ctx context.Context, providerName string, providerNamespace string) (out *api.DynamicProviderServer, err error) {
	// DynamicProviderServer CRs are always created in the controller namespace.
	// Provider CRs can be in any namespace, but their corresponding servers are always co-located
	// with the controller to enable service access and proper owner reference management.
	// We use label selectors to efficiently find servers that reference providers in other namespaces.
	serverList := &api.DynamicProviderServerList{}
	err = r.List(ctx, serverList, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(map[string]string{
			LabelProviderName:      providerName,
			LabelProviderNamespace: providerNamespace,
		}),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Find the first non-deleted server
	for i := range serverList.Items {
		item := &serverList.Items[i]
		if item.DeletionTimestamp == nil {
			out = item
			return
		}
	}
	// Not found
	return
}

// FindProviderServersByProvider finds ALL DynamicProviderServer CRs for a given Provider.
// Searches the controller namespace using label selectors to find servers that reference
// the specified provider (which may be in any namespace).
func (r *Ensurer) FindProviderServersByProvider(ctx context.Context, providerName, providerNamespace string) (servers []api.DynamicProviderServer, err error) {
	serverList := &api.DynamicProviderServerList{}
	err = r.List(ctx, serverList, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(map[string]string{
			LabelProviderName:      providerName,
			LabelProviderNamespace: providerNamespace,
		}),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	servers = serverList.Items
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
		err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.Server, pvc)
		if err != nil {
			return
		}
		err = r.Create(ctx, pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created PersistentVolumeClaim.", "pvc", pvc.Name, "server", r.Server.Name, "namespace", r.Server.Namespace)
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
		err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.Server, deployment)
		if err != nil {
			return
		}
		err = r.Create(ctx, deployment)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created Deployment.", "deployment", deployment.Name, "server", r.Server.Name, "namespace", r.Server.Namespace)
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
		err = r.Labeler.SetBlockingOwnerReference(r.Scheme(), r.Server, svc)
		if err != nil {
			return
		}
		err = r.Create(ctx, svc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created Service.", "service", svc.Name, "server", r.Server.Name, "namespace", r.Server.Namespace)
		out = svc
	} else {
		out = &list.Items[0]
	}
	return
}
