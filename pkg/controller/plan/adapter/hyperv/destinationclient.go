package hyperv

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DestinationClient struct {
	*plancontext.Context
}

func (r *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	if r.Source.Provider.GetHyperVTransferMethod() != api.HyperVTransferMethodISCSI {
		return nil
	}
	migrationUID := string(r.Migration.UID)
	list := api.HyperVVolumePopulatorList{}
	err := r.Destination.Client.List(context.TODO(), &list, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": migrationUID,
			"vmID":      vm.ID,
		}),
	})
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		if err := client.IgnoreNotFound(r.Destination.Client.Delete(context.TODO(), &list.Items[i])); err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

func (r *DestinationClient) SetPopulatorCrOwnership() error {
	return nil
}
