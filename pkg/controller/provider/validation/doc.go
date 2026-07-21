package validation

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	ec2validation "github.com/kubev2v/forklift/pkg/provider/ec2/controller/validation"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderValidator interface {
	Validate(provider *api.Provider, secret *core.Secret, client client.Client)
}

type NoopValidator struct{}

func (v *NoopValidator) Validate(_ *api.Provider, _ *core.Secret, _ client.Client) {}

func Build(provider *api.Provider) ProviderValidator {
	switch provider.Type() {
	case api.EC2:
		return &ec2validation.Validator{}
	default:
		return &NoopValidator{}
	}
}
