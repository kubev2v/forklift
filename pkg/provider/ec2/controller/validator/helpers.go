package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// getAWSInstance returns an EC2 instance from inventory.
func (r *Validator) getAWSInstance(vmRef ref.Ref) (*model.InstanceDetails, error) {
	return inventory.GetAWSInstance(r.Source.Inventory, vmRef)
}

// getBlockDevices returns block device mappings.
func getBlockDevices(awsInstance *model.InstanceDetails) ([]model.InstanceBlockDeviceMapping, bool) {
	return inventory.GetBlockDevices(awsInstance)
}

// getNetworkInterfaces returns network interfaces.
func getNetworkInterfaces(awsInstance *model.InstanceDetails) ([]model.InstanceNetworkInterface, bool) {
	return inventory.GetNetworkInterfaces(awsInstance)
}
