package admitters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	adapter "github.com/kubev2v/forklift/pkg/controller/plan/adapter/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	webhookutils "github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Application settings.
var Settings = &settings.Settings

var log = logging.WithName("admitter")

func init() {
	err := Settings.Inventory.Load()
	if err != nil {
		panic(err)
	}
}

type SecretAdmitter struct {
	ar     *admissionv1.AdmissionReview
	secret core.Secret
	Client client.Client
}

var resourceTypeToValidateFunc = map[string]func(*SecretAdmitter) *admissionv1.AdmissionResponse{
	"hosts": func(admitter *SecretAdmitter) *admissionv1.AdmissionResponse {
		return admitter.validateHostSecret()
	},
	"providers": func(admitter *SecretAdmitter) *admissionv1.AdmissionResponse {
		return admitter.validateProviderSecret()
	},
}

func (admitter *SecretAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("secret admitter was called")
	admitter.ar = ar
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.secret)
	if err != nil {
		log.Error(err, "secret webhook error, failed to unmarshel secret")
		return webhookutils.ToAdmissionResponseError(err)
	}

	// The label createdForResourceType must exist due to the configuration of the webhook
	resourceType := admitter.secret.GetLabels()["createdForResourceType"]
	if validate, ok := resourceTypeToValidateFunc[resourceType]; ok {
		return validate(admitter)
	}

	return webhookutils.ToAdmissionResponseAllow()
}

func (admitter *SecretAdmitter) validateProviderSecret() *admissionv1.AdmissionResponse {
	var insecure bool
	if val, ok := admitter.secret.Data["insecure"]; ok {
		var err error
		if insecure, err = strconv.ParseBool(string(val)); err != nil {
			return webhookutils.ToAdmissionResponseError(fmt.Errorf("invalid value for 'insecure' field"))
		}
	}

	if _, ok := admitter.secret.Data["cacert"]; ok && insecure {
		return webhookutils.ToAdmissionResponseError(fmt.Errorf("received a request to add insecure provider with a CA certificate"))
	}

	if createdForProviderType, ok := admitter.secret.GetLabels()["createdForProviderType"]; ok {
		providerType := api.ProviderType(createdForProviderType)

		if admitter.ar.Request.Operation == admissionv1.Update && providerType == api.Ova {
			// there's no need to proceed to provider connection test since the URL
			// does not change and credentials are not specified
			return admitter.validateUpdateOfOVAProviderSecret()
		}

		collector, err := admitter.buildProviderCollector(&providerType)
		if err != nil {
			return webhookutils.ToAdmissionResponseError(err)
		}

		log.Info("Starting provider connection test")
		if status, err := collector.Test(); err != nil {
			if status == http.StatusUnauthorized || status == http.StatusBadRequest {
				log.Info("Connection test failed, failing", "status", status)
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusForbidden,
						Message: "Invalid credentials",
					},
				}
			} else {
				log.Info("Connection test failed, yet passing", "status", status, "error", err.Error())
			}
		} else {
			log.Info("Test succeeded, passing")
		}
		return webhookutils.ToAdmissionResponseAllow()
	} else {
		err := errors.New("provider secret is labeled with 'createdForResourceType' but without 'createdForProviderType'")
		return webhookutils.ToAdmissionResponseError(err)
	}
}

func (admitter *SecretAdmitter) validateHostSecret() *admissionv1.AdmissionResponse {
	if hostName, ok := admitter.secret.GetLabels()["createdForResource"]; ok {
		if _, ok := admitter.secret.Data["user"]; !ok {
			err := errors.New("missing credentials on Host secret")
			return webhookutils.ToAdmissionResponseError(err)
		}
		tested, err := admitter.testConnectionToHost(hostName)
		switch {
		case tested && err != nil:
			log.Info("Test connection to the host failed")
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    http.StatusForbidden,
					Message: err.Error(),
				},
			}
		case err != nil:
			log.Info("Couldn't test connection to the host, yet passing", "error", err.Error())
		case tested:
			log.Info("Test succeeded, passing")
		}
		return webhookutils.ToAdmissionResponseAllow()
	} else {
		err := errors.New("host secret is labeled with 'createdForResourceType' but without 'createdForResource'")
		return webhookutils.ToAdmissionResponseError(err)
	}
}

func (admitter *SecretAdmitter) buildProviderCollector(providerType *api.ProviderType) (libcontainer.Collector, error) {
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      admitter.secret.Name,
			Namespace: admitter.secret.Namespace,
		},
		Spec: api.ProviderSpec{
			Type: providerType,
			URL:  string(admitter.secret.Data["url"]),
		},
	}

	if collector := container.Build(nil, provider, &admitter.secret); collector != nil {
		return collector, nil
	} else {
		return nil, fmt.Errorf("incorrect 'createdForProviderType' value. Options %s", api.ProviderTypes)
	}
}

func (admitter *SecretAdmitter) ensureEsxiCredentials(provider *api.Provider) (*core.Secret, error) {
	updatedSecret := admitter.secret.DeepCopy()
	user, ok := updatedSecret.Data["user"]

	// if the user is not set and the api type is ESXi, we need to copy the credentials from the provider
	if provider.Spec.Settings[api.SDK] == api.ESXI && (!ok || string(user) == "") {
		ref := provider.Spec.Secret
		providerSecret := &core.Secret{}
		if err := admitter.Client.Get(context.TODO(), client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, providerSecret); err != nil {
			log.Error(err, "failed to get provider secret for Host secret without credentials")
			return nil, err
		}

		// update the deep copy with user and password
		updatedSecret.Data["user"] = providerSecret.Data["user"]
		updatedSecret.Data["password"] = providerSecret.Data["password"]
	}

	return updatedSecret, nil
}

func (admitter *SecretAdmitter) testConnectionToHost(hostName string) (tested bool, err error) {
	provider := &api.Provider{}
	providerName := string(admitter.secret.Data["provider"])
	// there is an assumption that the provider resides within the same namespace as the secret
	// which is reasonable as the hosts are also created on the same namespace as the provider
	// but anyway, if that's not the case, we would likely pass the validation (due to IsNotFound check)
	providerNamespace := admitter.secret.Namespace
	err = admitter.Client.Get(context.TODO(), client.ObjectKey{Namespace: providerNamespace, Name: providerName}, provider)
	if err != nil {
		if k8serr.IsNotFound(err) {
			log.Info("Failed to find provider of host, passing")
			err = nil
			return
		} else {
			log.Error(err, "Couldn't get the target provider")
			return
		}
	}

	switch provider.Type() {
	case api.VSphere:
		inventory, err := web.NewClient(provider)
		if err != nil {
			return false, err
		}
		hostModel := &vsphere.Host{}
		err = inventory.Get(hostModel, hostName)
		if err != nil {
			return false, err
		}
		admitter.secret.Data["thumbprint"] = []byte(hostModel.Thumbprint)
		url := fmt.Sprintf("https://%s/sdk", admitter.secret.Data["ip"])

		updatedSecret, err := admitter.ensureEsxiCredentials(provider)
		if err != nil {
			return false, err
		}

		h := adapter.EsxHost{
			Secret: updatedSecret,
			URL:    url,
		}
		log.Info("Testing provider connection")
		return true, h.TestConnection()
	default:
		return true, nil
	}
}

func (admitter *SecretAdmitter) validateUpdateOfOVAProviderSecret() *admissionv1.AdmissionResponse {
	urlChanged, err := admitter.isOvaUrlChanged()
	if err != nil {
		return webhookutils.ToAdmissionResponseError(err)
	}

	if urlChanged {
		log.Info("reject changing the URL of an existing OVA provider")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: "Updating the URL field of an existing OVA provider is forbidden.",
			},
		}
	}

	return webhookutils.ToAdmissionResponseAllow()
}

func (admitter *SecretAdmitter) isOvaUrlChanged() (bool, error) {
	oldSecret := core.Secret{}
	err := admitter.Client.Get(context.TODO(), client.ObjectKey{Namespace: admitter.secret.Namespace, Name: admitter.secret.Name}, &oldSecret)
	if err != nil {
		log.Error(err, "Couldn't get the target provider secret")
		return false, err
	}

	url := oldSecret.Data["url"]
	newURL := admitter.secret.Data["url"]
	return !bytes.Equal(url, newURL), nil
}
