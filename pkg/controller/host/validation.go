package host

import (
	"context"
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	ProviderNotValid = "ProviderNotValid"
	HostNotValid     = "HostNotValid"
	SecretNotValid   = "SecretNotValid"
	TypeNotValid     = "TypeNotValid"
	IpNotValid       = "IpNotValid"
)

//
// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

//
// Reasons
const (
	NotSet    = "NotSet"
	NotFound  = "NotFound"
	DataErr   = "DataErr"
	TypeErr   = "TypeErr"
	Ambiguous = "Ambiguous"
)

//
// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Validate the mp resource.
func (r *Reconciler) validate(host *api.Host) error {
	err := r.validateProvider(host)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateRef(host)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateIp(host)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateSecret(host)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate provider field.
func (r *Reconciler) validateProvider(host *api.Host) error {
	pVal := validation.Provider{
		Client: r,
	}
	conditions, err := pVal.Validate(host.Spec.Provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	host.Status.UpdateConditions(conditions)
	if pVal.Referenced == nil {
		return nil
	}
	host.Referenced.Provider.Source = pVal.Referenced
	switch pVal.Referenced.Type() {
	case api.VSphere:
	default:
		host.Status.SetCondition(
			libcnd.Condition{
				Type:     TypeNotValid,
				Status:   True,
				Reason:   TypeErr,
				Category: Critical,
				Message:  "Provider type not supported.",
			})
	}

	return nil
}

//
// Validate host ref.
func (r *Reconciler) validateRef(host *api.Host) error {
	ref := host.Spec.Ref
	if ref.NotSet() {
		host.Status.SetCondition(
			libcnd.Condition{
				Type:     HostNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `id` is not valid.",
			})
		return nil
	}
	provider := host.Referenced.Provider.Source
	if provider == nil {
		return nil
	}
	inventory, err := web.NewClient(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	_, err = inventory.Host(&ref)
	if err != nil {
		if errors.Is(err, web.NotFoundErr) {
			host.Status.SetCondition(
				libcnd.Condition{
					Type:     HostNotValid,
					Status:   True,
					Reason:   NotFound,
					Category: Critical,
					Message:  "Referenced host not found.",
				})
			return nil
		}
		if errors.Is(err, web.RefNotUniqueErr) {
			host.Status.SetCondition(
				libcnd.Condition{
					Type:     HostNotValid,
					Status:   True,
					Reason:   Ambiguous,
					Category: Critical,
					Message:  "Host reference is ambiguous.",
				})
			return nil
		}
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate host ID field.
func (r *Reconciler) validateIp(host *api.Host) error {
	if host.Spec.IpAddress == "" {
		host.Status.SetCondition(
			libcnd.Condition{
				Type:     IpNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `ipAddress` is not valid.",
			})
	}

	return nil
}

//
// Validate secret (ref).
//   1. The references is complete.
//   2. The secret exists.
//   3. the content of the secret is valid.
func (r *Reconciler) validateSecret(host *api.Host) error {
	ref := host.Spec.Secret
	if !libref.RefSet(ref) {
		return nil
	}
	newCnd := libcnd.Condition{
		Type:     SecretNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "The `secret` is not valid.",
	}
	// NotFound
	secret := &core.Secret{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := r.Get(context.TODO(), key, secret)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		host.Status.SetCondition(newCnd)
		return nil
	}
	if err != nil {
		return liberr.Wrap(err)
	}
	// DataErr
	keyList := []string{}
	provider := host.Referenced.Provider.Source
	if provider != nil {
		switch provider.Type() {
		case api.VSphere:
			keyList = []string{
				"user",
				"password",
			}
		}
	}
	for _, key := range keyList {
		if _, found := secret.Data[key]; !found {
			newCnd.Items = append(newCnd.Items, key)
		}
	}
	if len(newCnd.Items) > 0 {
		newCnd.Reason = DataErr
		host.Status.SetCondition(newCnd)
	}

	return nil
}
