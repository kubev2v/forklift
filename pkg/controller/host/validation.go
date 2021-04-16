package host

import (
	"context"
	"errors"
	"fmt"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	builder "github.com/konveyor/forklift-controller/pkg/controller/plan/builder/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	Validated               = "Validated"
	RefNotValid             = "RefNotValid"
	SecretNotValid          = "SecretNotValid"
	TypeNotValid            = "TypeNotValid"
	IpNotValid              = "IpNotValid"
	ConnectionTestSucceeded = "ConnectionTestSucceeded"
	ConnectionTestFailed    = "ConnectionTestFailed"
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
	Completed = "Completed"
	Tested    = "Tested"
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
	host.Status.SetCondition(
		libcnd.Condition{
			Type:     Validated,
			Status:   True,
			Reason:   Completed,
			Category: Advisory,
			Message:  "The host has been validated.",
		})
	err = r.testConnection(host)
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
				Type:     RefNotValid,
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
		if errors.As(err, &web.NotFoundError{}) {
			host.Status.SetCondition(
				libcnd.Condition{
					Type:     RefNotValid,
					Status:   True,
					Reason:   NotFound,
					Category: Critical,
					Message:  "Referenced host not found.",
				})
			return nil
		}
		if errors.As(err, &web.RefNotUniqueError{}) {
			host.Status.SetCondition(
				libcnd.Condition{
					Type:     RefNotValid,
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
func (r *Reconciler) validateSecret(host *api.Host) (err error) {
	ref := host.Spec.Secret
	cnd := libcnd.Condition{
		Type:     SecretNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "The `secret` is not set.",
	}
	if !libref.RefSet(&ref) {
		host.Status.SetCondition(cnd)
		return
	}
	// NotFound
	secret := &core.Secret{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err = r.Get(context.TODO(), key, secret)
	if k8serr.IsNotFound(err) {
		err = nil
		cnd.Reason = NotFound
		cnd.Message = "The `secret` cannot be found."
		host.Status.SetCondition(cnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	host.Referenced.Secret = secret
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
			cnd.Items = append(cnd.Items, key)
		}
	}
	if len(cnd.Items) > 0 {
		cnd.Reason = DataErr
		cnd.Message = "The `secret` missing required data."
		host.Status.SetCondition(cnd)
	}

	return
}

//
// Test connection.
func (r *Reconciler) testConnection(host *api.Host) (err error) {
	if host.Status.HasBlockerCondition() {
		return
	}
	provider := host.Referenced.Provider.Source
	secret := host.Referenced.Secret
	inventory, err := web.NewClient(provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var testErr error
	switch provider.Type() {
	case api.VSphere:
		url := fmt.Sprintf("https://%s/sdk", host.Spec.IpAddress)
		hostModel := &vsphere.Host{}
		pErr := inventory.Find(hostModel, host.Spec.Ref)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		secret.Data["thumbprint"] = []byte(hostModel.Thumbprint)
		h := builder.EsxHost{
			Secret: secret,
			URL:    url,
		}
		r.Log.V(1).Info(
			"Testing connection.",
			"url",
			url)
		testErr = h.TestConnection()
		if testErr != nil {
			r.Log.V(1).Info(
				"Connection test, failed",
				"url",
				url,
				"reason",
				testErr.Error())
		}
	default:
		return
	}
	if testErr == nil {
		host.Status.SetCondition(
			libcnd.Condition{
				Type:     ConnectionTestSucceeded,
				Status:   True,
				Reason:   Tested,
				Category: Required,
				Message:  "Connection test, succeeded.",
			})
	} else {
		host.Status.SetCondition(
			libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message: fmt.Sprintf(
					"Connection test, failed: %s",
					testErr.Error()),
			})
	}

	return
}
