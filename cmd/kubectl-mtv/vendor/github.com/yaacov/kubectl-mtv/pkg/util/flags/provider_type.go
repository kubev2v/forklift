package flags

import (
	"fmt"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// staticProviderTypes is the single source of truth for built-in provider types.
var staticProviderTypes = []forkliftv1beta1.ProviderType{
	forkliftv1beta1.OpenShift,
	forkliftv1beta1.VSphere,
	forkliftv1beta1.OVirt,
	forkliftv1beta1.OpenStack,
	forkliftv1beta1.Ova,
	forkliftv1beta1.HyperV,
	forkliftv1beta1.EC2,
}

// staticProviderTypeStrings returns the string representations of static provider types.
func staticProviderTypeStrings() []string {
	strs := make([]string, len(staticProviderTypes))
	for i, t := range staticProviderTypes {
		strs[i] = string(t)
	}
	return strs
}

// ProviderTypeFlag implements pflag.Value interface for provider type validation
type ProviderTypeFlag struct {
	value        string
	dynamicTypes []string
}

func (p *ProviderTypeFlag) String() string {
	return p.value
}

func (p *ProviderTypeFlag) Set(value string) error {
	// Check static types
	isValid := false
	for _, validType := range staticProviderTypes {
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
		validTypesStr := strings.Join(staticProviderTypeStrings(), ", ")
		if len(p.dynamicTypes) > 0 {
			validTypesStr = fmt.Sprintf("%s, %s", validTypesStr, strings.Join(p.dynamicTypes, ", "))
		}
		return fmt.Errorf("invalid provider type: %s. Valid types are: %s", value, validTypesStr)
	}

	p.value = value
	return nil
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
	staticStrs := staticProviderTypeStrings()

	// Combine static and dynamic types
	allTypes := make([]string, 0, len(staticStrs)+len(p.dynamicTypes))
	allTypes = append(allTypes, staticStrs...)
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
