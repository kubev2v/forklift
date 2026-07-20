package nutanix

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

// No destination setup/cleanup needed for Nutanix.
type DestinationClient struct {
	*plancontext.Context
}

func (r *DestinationClient) DeletePopulatorDataSource(_ *plan.VMStatus) error {
	return nil
}

func (r *DestinationClient) SetPopulatorCrOwnership() error {
	return nil
}
