package validation

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	ec2validation "github.com/kubev2v/forklift/pkg/provider/ec2/controller/validation"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Runner is implemented by the provider reconciler so provider-specific
// validation methods can be invoked from this package without an import cycle.
type Runner interface {
	ValidateVSpherePrivileges(provider *api.Provider) error
	ValidateSSHReadiness(provider *api.Provider, secret *core.Secret) error
	ValidateSMBCSI(provider *api.Provider) error
	ValidateHyperVSettings(provider *api.Provider) error
}

// ProviderValidator runs provider-type-specific validation.
type ProviderValidator struct {
	runner Runner
	client client.Client
}

func Build(runner Runner, cl client.Client) *ProviderValidator {
	return &ProviderValidator{runner: runner, client: cl}
}

func (v *ProviderValidator) Validate(provider *api.Provider, secret *core.Secret) error {
	switch provider.Type() {
	case api.EC2:
		(&ec2validation.Validator{}).Validate(provider, secret, v.client)
		return nil
	case api.VSphere:
		err := v.runner.ValidateVSpherePrivileges(provider)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = v.runner.ValidateSSHReadiness(provider, secret)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	case api.HyperV:
		err := v.runner.ValidateSMBCSI(provider)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = v.runner.ValidateHyperVSettings(provider)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	default:
		return nil
	}
}
