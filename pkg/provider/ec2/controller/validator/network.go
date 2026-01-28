package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/mapping"
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

	for _, iface := range interfaces {
		if iface.SubnetId == nil {
			continue
		}

		if !mapping.HasNetworkMapping(r.Map.Network, *iface.SubnetId) {
			return false, nil
		}
	}

	return true, nil
}
