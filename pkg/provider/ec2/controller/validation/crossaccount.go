package validation

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TargetCredentialsMissing = "TargetCredentialsMissing"
	ValidationFailed         = "ValidationFailed"
)

type Validator struct{}

func (v *Validator) Validate(provider *api.Provider, secret *core.Secret, _ client.Client) {
	if provider.Status.HasBlockerCondition() {
		return
	}
	if secret == nil {
		return
	}

	_, hasKeyID := secret.Data["targetAccessKeyId"]
	_, hasSecret := secret.Data["targetSecretAccessKey"]

	if !hasKeyID || !hasSecret {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(libcnd.Condition{
			Type:     TargetCredentialsMissing,
			Status:   libcnd.True,
			Category: libcnd.Critical,
			Reason:   "TargetCredentialsRequired",
			Message:  "The provider secret must include 'targetAccessKeyId' and 'targetSecretAccessKey'. Recreate the provider with --auto-target-credentials or provide them manually.",
		})
	} else {
		provider.Status.DeleteCondition(TargetCredentialsMissing)
	}
}
