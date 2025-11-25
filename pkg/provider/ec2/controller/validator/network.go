package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NetworksMapped validates all EC2 network interfaces have subnet mappings configured.
// Checks each network interface's SubnetId against network mapping.
// Returns error listing any unmapped subnets that would block migration.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (bool, error) {
	awsInstance, err := r.getAWSInstance(vmRef)
	if err != nil {
		return false, err
	}

	interfaces, found := getNetworkInterfaces(awsInstance)
	if !found {
		return true, nil
	}

	for _, ifaceIface := range interfaces {
		iface, ok := ifaceIface.(map[string]interface{})
		if !ok {
			continue
		}

		subnetID, _, _ := unstructured.NestedString(iface, "SubnetId")
		if !r.hasNetworkMapping(subnetID) {
			return false, nil
		}
	}

	return true, nil
}

// hasNetworkMapping checks if a subnet has a mapping.
func (r *Validator) hasNetworkMapping(subnetID string) bool {
	if r.Context.Map.Network == nil || r.Context.Map.Network.Spec.Map == nil {
		return false
	}

	for _, mapping := range r.Context.Map.Network.Spec.Map {
		if mapping.Source.ID == subnetID {
			return true
		}
	}

	return false
}
