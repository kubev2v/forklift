package ocp

import (
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
)

type DestinsationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource implements base.DestinationClient
func (*DestinsationClient) DeletePopulatorDataSource(vm *planapi.VMStatus) error {
	return nil
}

// SetPopulatorCrOwnership implements base.DestinationClient
func (*DestinsationClient) SetPopulatorCrOwnership() error {
	return nil
}
