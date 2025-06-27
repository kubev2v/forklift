package provider

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	UrlNotValid             = "UrlNotValid"
	TypeNotSupported        = "ProviderTypeNotSupported"
	SecretNotValid          = "SecretNotValid"
	SettingsNotValid        = "SettingsNotValid"
	Validated               = "Validated"
	ConnectionAuthFailed    = "ConnectionAuthFailed"
	ConnectionTestSucceeded = "ConnectionTestSucceeded"
	ConnectionTestFailed    = "ConnectionTestFailed"
	InventoryCreated        = "InventoryCreated"
	LoadInventory           = "LoadInventory"
	ConnectionInsecure      = "ConnectionInsecure"
)

// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

// Reasons
const (
	NotSet              = "NotSet"
	NotFound            = "NotFound"
	NotSupported        = "NotSupported"
	DataErr             = "DataErr"
	Malformed           = "Malformed"
	Completed           = "Completed"
	Tested              = "Tested"
	Started             = "Started"
	SkipTLSVerification = "SkipTLSVerification"
)

// Phases
const (
	ValidationFailed = "ValidationFailed"
	ConnectionFailed = "ConnectionFailed"
	Ready            = "Ready"
	Staging          = "Staging"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

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
	secret, err := r.validateSecret(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.testConnection(provider, secret)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.inventoryCreated(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	if !provider.Status.HasBlockerCondition() {
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     Validated,
				Status:   True,
				Reason:   Completed,
				Category: Advisory,
				Message:  "Validation has been completed.",
			})
	}

	return nil
}

// Validate types.
func (r *Reconciler) validateType(provider *api.Provider) error {
	for _, p := range api.ProviderTypes {
		if p == provider.Type() {
			return nil
		}
	}

	provider.Status.Phase = ValidationFailed
	provider.Status.SetCondition(
		libcnd.Condition{
			Type:     TypeNotSupported,
			Status:   True,
			Reason:   NotSupported,
			Category: Critical,
			Message:  fmt.Sprintf("The `type` must be: %s", api.ProviderTypes),
		})

	return nil
}

// Validate the URL.
func (r *Reconciler) validateURL(provider *api.Provider) error {
	if provider.IsHost() {
		return nil
	}
	if provider.Spec.URL == "" {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     UrlNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `url` is not valid.",
			})
	}
	if provider.Type() == api.Ova {
		if !isValidNFSPath(provider.Spec.URL) {
			provider.Status.Phase = ValidationFailed
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     UrlNotValid,
					Status:   True,
					Reason:   Malformed,
					Category: Critical,
					Message:  "The NFS path is malformed",
				})
		}
		return nil
	}
	_, err := url.Parse(provider.Spec.URL)
	if err != nil {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     UrlNotValid,
				Status:   True,
				Reason:   Malformed,
				Category: Critical,
				Message:  fmt.Sprintf("The `url` is malformed: %s", err.Error()),
			})
	}

	return nil
}

func (r *Reconciler) validateConnectionStatus(provider *api.Provider, secret *core.Secret) {
	if base.GetInsecureSkipVerifyFlag(secret) {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     ConnectionInsecure,
			Status:   True,
			Reason:   SkipTLSVerification,
			Category: Warn,
			Message:  "TLS is susceptible to machine-in-the-middle attacks when certificate verification is skipped.",
		})
	} else {
		_, err := base.VerifyTLSConnection(provider.Spec.URL, secret)
		if err != nil {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message:  err.Error(),
			})
		}
	}
}

// Validate secret (ref).
//  1. The references is complete.
//  2. The secret exists.
//  3. the content of the secret is valid.
func (r *Reconciler) validateSecret(provider *api.Provider) (secret *core.Secret, err error) {
	if provider.IsHost() {
		return
	}
	// NotSet
	newCnd := libcnd.Condition{
		Type:     SecretNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "The `secret` is not valid.",
	}
	ref := provider.Spec.Secret
	if !libref.RefSet(&ref) {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(newCnd)
		return
	}
	// NotFound
	secret = &core.Secret{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err = r.Get(context.TODO(), key, secret)
	if k8serrors.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	// DataErr
	keyList := []string{}
	switch provider.Type() {
	case api.OpenShift:
		keyList = []string{"token"}

	case api.VSphere:
		keyList = []string{
			"user",
			"password",
		}

		r.validateConnectionStatus(provider, secret)

		var providerUrl *url.URL
		providerUrl, err = url.Parse(provider.Spec.URL)
		if err != nil {
			return
		}
		var crt *x509.Certificate
		crt, err = util.GetTlsCertificate(providerUrl, secret)
		if err != nil {
			provider.Status.Phase = ConnectionFailed
			provider.Status.SetCondition(libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message:  err.Error(),
			})
			return
		}
		provider.Status.Fingerprint = util.Fingerprint(crt)
	case api.OVirt:
		keyList = []string{
			"user",
			"password",
		}

		if base.GetInsecureSkipVerifyFlag(secret) {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     ConnectionInsecure,
				Status:   True,
				Reason:   SkipTLSVerification,
				Category: Warn,
				Message:  "TLS is susceptible to machine-in-the-middle attacks when certificate verification is skipped.",
			})
		} else {
			keyList = append(keyList, "cacert")
		}
	case api.Ova:
		keyList = []string{
			"url",
		}
	}
	for _, key := range keyList {
		if _, found := secret.Data[key]; !found {
			newCnd.Items = append(newCnd.Items, key)
		}
	}
	if len(newCnd.Items) > 0 {
		newCnd.Reason = DataErr
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(newCnd)
	}

	return
}

// Test connection.
func (r *Reconciler) testConnection(provider *api.Provider, secret *core.Secret) error {
	if provider.Status.HasBlockerCondition() {
		return nil
	}
	rl := container.Build(nil, provider, secret)
	status, err := rl.Test()
	if err == nil {
		log.Info(
			"Connection test succeeded.")
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     ConnectionTestSucceeded,
				Status:   True,
				Reason:   Tested,
				Category: Required,
				Message:  "Connection test, succeeded.",
			})
	} else {
		// When the status is unauthorized controller stops the reconciliation, so the user account does not get locked.
		// Providing bad credentials when requesting the token results in 400, and not 401.
		if status == http.StatusUnauthorized || status == http.StatusBadRequest {
			provider.Status.Phase = ConnectionFailed
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     ConnectionAuthFailed,
					Status:   True,
					Reason:   Tested,
					Category: Critical,
					Message: fmt.Sprintf(
						"Connection auth failed, error: %s",
						err.Error()),
				})
			return nil
		}
		log.Info(
			"Connection test failed.",
			"reason",
			err.Error())
		provider.Status.Phase = ConnectionFailed
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message: fmt.Sprintf(
					"Connection test, failed: %s",
					err.Error()),
			})
	}

	return nil
}

// Validate inventory created.
func (r *Reconciler) inventoryCreated(provider *api.Provider) error {
	if provider.Status.HasBlockerCondition() {
		return nil
	}
	if r, found := r.container.Get(provider); found {
		if r.HasParity() {
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     InventoryCreated,
					Status:   True,
					Reason:   Completed,
					Category: Required,
					Message:  "The inventory has been loaded.",
				})
		} else {
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     LoadInventory,
					Status:   True,
					Reason:   Started,
					Category: Advisory,
					Message:  "Loading the inventory.",
				})
		}
	}

	return nil
}

func (r *Reconciler) handleServerCreationFailure(provider *api.Provider, err error) {
	provider.Status.Phase = ConnectionFailed
	msg := fmt.Sprint("The OVA provider server creation failed: ", err)
	provider.Status.SetCondition(
		libcnd.Condition{
			Type:     ConnectionFailed,
			Status:   True,
			Category: Critical,
			Message:  msg,
		})
	if updateErr := r.Status().Update(context.TODO(), provider.DeepCopy()); updateErr != nil {
		log.Error(updateErr, "Failed to update provider status")
	}
}

func isValidNFSPath(nfsPath string) bool {
	nfsRegex := `^[^:]+:\/[^:].*$`
	re := regexp.MustCompile(nfsRegex)
	return re.MatchString(nfsPath)
}
