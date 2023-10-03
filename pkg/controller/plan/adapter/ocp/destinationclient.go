package ocp

import (
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
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
