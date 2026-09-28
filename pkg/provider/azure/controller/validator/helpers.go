package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

func (r *Validator) getAzureVM(vmRef ref.Ref) (*model.VMDetails, error) {
	return inventory.GetAzureVM(r.Source.Inventory, vmRef)
}
