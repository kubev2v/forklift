package flags

import (
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// ProviderTypeFlag implements pflag.Value interface for provider type validation
type ProviderTypeFlag struct {
	value        string
	dynamicTypes []string
}

func (p *ProviderTypeFlag) String() string {
	return p.value
}

func (p *ProviderTypeFlag) Set(value string) error {
	// Static provider types
	staticTypes := []forkliftv1beta1.ProviderType{
		forkliftv1beta1.OpenShift,
		forkliftv1beta1.VSphere,
		forkliftv1beta1.OVirt,
		forkliftv1beta1.OpenStack,
		forkliftv1beta1.Ova,
		"hyperv",
		"ec2",
	}

	// Check static types
	isValid := false
	for _, validType := range staticTypes {
		if forkliftv1beta1.ProviderType(value) == validType {
			isValid = true
			break
		}
	}

	// Check dynamic types if not found in static types
	if !isValid {
		for _, dynamicType := range p.dynamicTypes {
			if value == dynamicType {
				isValid = true
				break
			}
		}
	}

	if !isValid {
		validTypesStr := "openshift, vsphere, ovirt, openstack, ova, hyperv, ec2"
		if len(p.dynamicTypes) > 0 {
			validTypesStr = fmt.Sprintf("%s, %s", validTypesStr, joinStrings(p.dynamicTypes, ", "))
		}
		return fmt.Errorf("invalid provider type: %s. Valid types are: %s", value, validTypesStr)
	}

	p.value = value
	return nil
}

// joinStrings joins a slice of strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func (p *ProviderTypeFlag) Type() string {
	return "string"
}

// GetValue returns the provider type value
func (p *ProviderTypeFlag) GetValue() string {
	return p.value
}

// GetValidValues returns all valid provider type values for auto-completion
func (p *ProviderTypeFlag) GetValidValues() []string {
	staticTypes := []string{"openshift", "vsphere", "ovirt", "openstack", "ova", "hyperv", "ec2"}

	// Combine static and dynamic types
	allTypes := make([]string, 0, len(staticTypes)+len(p.dynamicTypes))
	allTypes = append(allTypes, staticTypes...)
	allTypes = append(allTypes, p.dynamicTypes...)

	return allTypes
}

// SetDynamicTypes sets the list of dynamic provider types from the cluster
func (p *ProviderTypeFlag) SetDynamicTypes(types []string) {
	p.dynamicTypes = types
}

// NewProviderTypeFlag creates a new provider type flag
func NewProviderTypeFlag() *ProviderTypeFlag {
	return &ProviderTypeFlag{
		dynamicTypes: []string{},
	}
}
