package nutanix

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DestinationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource removes the NutanixNfsVolumePopulator CRs created
// for a VM's disks. The target PVCs are cleaned up generically by the
// migration controller; the populator pods by DeletePopulatorPods.
func (r *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	list := &api.NutanixNfsVolumePopulatorList{}
	err := r.Destination.List(
		context.TODO(),
		list,
		client.InNamespace(r.Plan.Spec.TargetNamespace),
		client.MatchingLabels{
			"migration": string(r.Migration.UID),
			"vmID":      vm.ID,
		})
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		cr := &list.Items[i]
		if err := r.Destination.Delete(context.TODO(), cr); err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
	}
	return nil
}

// SetPopulatorCrOwnership is a no-op: owner references are set at creation
// time in Builder.ensurePopulatorPVCAndCR.
func (r *DestinationClient) SetPopulatorCrOwnership() error {
	return nil
}
