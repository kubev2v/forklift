package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/mapping"
)

func (r *Validator) NetworksMapped(vmRef ref.Ref) (bool, error) {
	azureVM, err := r.getAzureVM(vmRef)
	if err != nil {
		return false, err
	}

	nicIDs := inventory.GetNetworkInterfaceIDs(azureVM)
	if len(nicIDs) == 0 {
		return true, nil
	}

	for _, nicID := range nicIDs {
		if !mapping.HasNetworkMapping(r.Map.Network, nicID) {
			return false, nil
		}
	}

	return true, nil
}
