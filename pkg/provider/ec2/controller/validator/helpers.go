package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	ec2controller "github.com/kubev2v/forklift/pkg/provider/ec2/controller"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// getAWSInstance returns an EC2 instance from inventory.
func (r *Validator) getAWSInstance(vmRef ref.Ref) (map[string]interface{}, error) {
	instance := &unstructured.Unstructured{}
	instance.SetUnstructuredContent(map[string]interface{}{"kind": "Instance"})

	err := r.Source.Inventory.Find(instance, vmRef)
	if err != nil {
		return nil, err
	}

	return ec2controller.GetAWSObject(instance)
}

// getBlockDevices returns block device mappings.
func getBlockDevices(awsInstance map[string]interface{}) ([]interface{}, bool) {
	blockDevices, found, _ := unstructured.NestedSlice(awsInstance, "BlockDeviceMappings")
	return blockDevices, found && len(blockDevices) > 0
}

// getNetworkInterfaces returns network interfaces.
func getNetworkInterfaces(awsInstance map[string]interface{}) ([]interface{}, bool) {
	interfaces, found, _ := unstructured.NestedSlice(awsInstance, "NetworkInterfaces")
	return interfaces, found && len(interfaces) > 0
}
