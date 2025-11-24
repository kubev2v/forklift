package provider

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/ova"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (r Reconciler) EnsureOVAProviderServer(ctx context.Context, provider *api.Provider) (err error) {
	builder := ova.Builder{}
	ensurer := ova.Ensurer{Client: r.Client, Log: r.Log}
	server := builder.ProviderServer(provider)
	server, err = ensurer.ProviderServer(ctx, server)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Convert ObjectReference to ServiceEndpoint
	if server.Status.Service != nil {
		servicePort := int32(8080) // Default OVA server port
		provider.Status.Service = &api.ServiceEndpoint{
			Name:      server.Status.Service.Name,
			Namespace: server.Status.Service.Namespace,
			Port:      &servicePort,
		}
	}

	cnd := server.Status.FindCondition(ova.ApplianceManagementEnabled)
	if cnd != nil {
		provider.Status.SetCondition(*cnd)
	}
	return
}

func (r Reconciler) DeleteOVAProviderServer(ctx context.Context, provider *api.Provider) (err error) {
	labeler := ova.Labeler{}
	list := &api.OVAProviderServerList{}
	err = r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(labeler.ProviderLabels(provider)),
		Namespace:     Settings.Namespace,
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	propagation := v1.DeletePropagationForeground
	for i := range list.Items {
		item := &list.Items[i]
		err = r.Delete(ctx, item, &k8sclient.DeleteOptions{PropagationPolicy: &propagation})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = nil
				continue
			}
			err = liberr.Wrap(err)
			return
		}
	}
	return
}
