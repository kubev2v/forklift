package provider

import (
	"context"
	"fmt"
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	UrlNotValid      = "UrlNotValid"
	TypeNotSupported = "ProviderTypeNotSupported"
	SecretNotValid   = "SecretNotValid"
)

//
// Categories
const (
	Required = cnd.Required
	Advisory = cnd.Advisory
	Critical = cnd.Critical
	Error    = cnd.Error
	Warn     = cnd.Warn
)

//
// Reasons
const (
	NotSet       = "NotSet"
	NotFound     = "NotFound"
	NotSupported = "NotSupported"
)

//
// Statuses
const (
	True  = cnd.True
	False = cnd.False
)

//
// Validate the provider resource.
func (r *Reconciler) validate(provider *api.Provider) error {
	err := r.validateType(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateURL(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateSecret(provider)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate types.
func (r *Reconciler) validateType(provider *api.Provider) error {
	switch provider.Type() {
	case api.OpenShift,
		api.VSphere:
	default:
		valid := []string{
			api.OpenShift,
			api.VSphere,
		}
		provider.Status.SetCondition(
			cnd.Condition{
				Type:     TypeNotSupported,
				Status:   True,
				Reason:   NotSupported,
				Category: Critical,
				Message:  fmt.Sprintf("The `type` must be: %s", valid),
			})
	}

	return nil
}

//
// Validate the URL.
func (r *Reconciler) validateURL(provider *api.Provider) error {
	if provider.IsHost() {
		return nil
	}
	if provider.Spec.URL == "" {
		provider.Status.SetCondition(
			cnd.Condition{
				Type:     UrlNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `url` is not valid.",
			})
	}

	return nil
}

//
// Validate secret (ref).
//   1. The references is complete.
//   2. The secret exists.
//   3. the content of the secret is valid.
func (r *Reconciler) validateSecret(provider *api.Provider) error {
	if provider.IsHost() {
		return nil
	}
	newCnd := cnd.Condition{
		Type:     SecretNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "The `secret` is not valid.",
	}
	ref := provider.Spec.Secret
	if !libref.RefSet(&ref) {
		provider.Status.SetCondition(newCnd)
		return nil
	}
	secret := &core.Secret{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := r.Get(context.TODO(), key, secret)
	if errors.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		provider.Status.SetCondition(newCnd)
	}
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}
