package dynamic

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

// DestinationClient provides access to the destination cluster.
type DestinationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource deletes populator data sources.
func (d *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	// Not supported for dynamic providers
	return nil
}

// SetPopulatorCrOwnership sets ownership on populator CRs.
func (r *DestinationClient) SetPopulatorCrOwnership() (err error) {
	// Not supported for dynamic providers
	return
}
