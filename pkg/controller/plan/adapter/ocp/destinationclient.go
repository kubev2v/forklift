package ocp

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

type DestinationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource implements base.DestinationClient
func (*DestinationClient) DeletePopulatorDataSource(vm *planapi.VMStatus) error {
	return nil
}

// SetPopulatorCrOwnership implements base.DestinationClient
func (*DestinationClient) SetPopulatorCrOwnership() error {
	return nil
}
