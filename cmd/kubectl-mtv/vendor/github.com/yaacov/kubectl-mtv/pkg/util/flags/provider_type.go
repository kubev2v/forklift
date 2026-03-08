package flags

import (
	"fmt"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// providerTypes is the single source of truth for built-in provider types.
var providerTypes = []forkliftv1beta1.ProviderType{
	forkliftv1beta1.OpenShift,
	forkliftv1beta1.VSphere,
	forkliftv1beta1.OVirt,
	forkliftv1beta1.OpenStack,
	forkliftv1beta1.Ova,
	forkliftv1beta1.HyperV,
	forkliftv1beta1.EC2,
}

// providerTypeStrings returns the string representations of provider types.
func providerTypeStrings() []string {
	strs := make([]string, len(providerTypes))
	for i, t := range providerTypes {
		strs[i] = string(t)
	}
	return strs
}

// ProviderTypeFlag implements pflag.Value interface for provider type validation
type ProviderTypeFlag struct {
	value string
}

func (p *ProviderTypeFlag) String() string {
	return p.value
}

func (p *ProviderTypeFlag) Set(value string) error {
	for _, validType := range providerTypes {
		if forkliftv1beta1.ProviderType(value) == validType {
			p.value = value
			return nil
		}
	}

	return fmt.Errorf("invalid provider type: %s. Valid types are: %s", value, strings.Join(providerTypeStrings(), ", "))
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
	return providerTypeStrings()
}

// NewProviderTypeFlag creates a new provider type flag
func NewProviderTypeFlag() *ProviderTypeFlag {
	return &ProviderTypeFlag{}
}
