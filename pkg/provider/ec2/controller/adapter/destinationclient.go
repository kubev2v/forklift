package adapter

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

// DestinationClient implements the base.DestinationClient interface for EC2.
// EC2 provider uses direct volume creation, so populator-related methods are no-ops.
type DestinationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource is a no-op for EC2.
// EC2 provider uses direct volume creation, not volume populators.
func (r *DestinationClient) DeletePopulatorDataSource(vm *planapi.VMStatus) error {
	return nil
}

// SetPopulatorCrOwnership is a no-op for EC2.
// EC2 provider uses direct volume creation, not volume populators.
func (r *DestinationClient) SetPopulatorCrOwnership() error {
	return nil
}
