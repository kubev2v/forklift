package inventory

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/web"
)

// Inventory defines the interface for inventory lookup operations.
// This matches the Source.Inventory field from plancontext.Context.
type Inventory interface {
	Find(resource interface{}, ref ref.Ref) error
}

// GetAWSInstance fetches an EC2 instance from the provider inventory.
// Returns ErrNoAWSInstanceObject if the instance data is missing.
func GetAWSInstance(inv Inventory, vmRef ref.Ref) (*model.InstanceDetails, error) {
	vm := &web.VM{}
	err := inv.Find(vm, vmRef)
	if err != nil {
		return nil, err
	}

	if vm.Object == nil {
		return nil, ErrNoAWSInstanceObject
	}

	return vm.Object, nil
}

// GetBlockDevices returns block device mappings from an EC2 instance.
// Returns the devices and a boolean indicating if any devices exist.
func GetBlockDevices(instance *model.InstanceDetails) ([]model.InstanceBlockDeviceMapping, bool) {
	return instance.BlockDeviceMappings, len(instance.BlockDeviceMappings) > 0
}

// GetNetworkInterfaces returns network interfaces from an EC2 instance.
// Returns the interfaces and a boolean indicating if any interfaces exist.
func GetNetworkInterfaces(instance *model.InstanceDetails) ([]model.InstanceNetworkInterface, bool) {
	return instance.NetworkInterfaces, len(instance.NetworkInterfaces) > 0
}

// GetInstanceName returns the instance name, falling back to instance ID if name is empty.
func GetInstanceName(instance *model.InstanceDetails) string {
	if instance.Name != "" {
		return instance.Name
	}
	if instance.InstanceId != nil {
		return *instance.InstanceId
	}
	return ""
}

// GetInstanceID returns the instance ID or empty string if nil.
func GetInstanceID(instance *model.InstanceDetails) string {
	if instance.InstanceId != nil {
		return *instance.InstanceId
	}
	return ""
}
