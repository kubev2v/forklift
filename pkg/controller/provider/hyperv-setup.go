package provider

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/hyperv"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (r Reconciler) EnsureHyperVProviderServer(ctx context.Context, provider *api.Provider) (err error) {
	builder := hyperv.Builder{}
	ensurer := hyperv.Ensurer{Client: r.Client, Log: r.Log}
	server := builder.ProviderServer(provider)
	server, err = ensurer.ProviderServer(ctx, server)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	provider.Status.Service = server.Status.Service
	cnd := server.Status.FindCondition(hyperv.ApplianceManagementEnabled)
	if cnd != nil {
		provider.Status.SetCondition(*cnd)
	}
	return
}

func (r Reconciler) DeleteHyperVProviderServer(ctx context.Context, provider *api.Provider) (err error) {
	labeler := hyperv.Labeler{}
	list := &api.HyperVProviderServerList{}
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
