package adapter

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

type DestinationClient struct {
	*plancontext.Context
}

func (r *DestinationClient) DeletePopulatorDataSource(vm *planapi.VMStatus) error {
	return nil
}

func (r *DestinationClient) SetPopulatorCrOwnership() error {
	return nil
}
