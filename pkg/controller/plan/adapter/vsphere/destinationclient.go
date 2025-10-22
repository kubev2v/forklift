package vsphere

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

type DestinationClient struct {
	*plancontext.Context
}

func (d *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	// not supported - do nothing
	return nil
}

func (r *DestinationClient) SetPopulatorCrOwnership() (err error) {
	// not supported - do nothing
	return
}
