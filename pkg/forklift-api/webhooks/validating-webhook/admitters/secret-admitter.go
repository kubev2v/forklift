package admitters

import (
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container"
	webhookutils "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	libcontainer "github.com/konveyor/forklift-controller/pkg/lib/inventory/container"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logging.WithName("admitter")

type SecretAdmitter struct {
	secret core.Secret
}

func (admitter *SecretAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("secret admitter was called")
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.secret)
	if err != nil {
		log.Error(err, "secret webhook error, failed to unmarshel secret")
		return webhookutils.ToAdmissionResponseError(err)
	}

	if createdForProviderType, ok := admitter.secret.GetLabels()["createdForProviderType"]; ok {
		providerType := api.ProviderType(createdForProviderType)
		collector, err := admitter.buildProviderCollector(&providerType)
		if err != nil {
			return webhookutils.ToAdmissionResponseError(err)
		}

		log.Info("Starting provider connection test")
		status, err := collector.Test()
		switch {
		case err != nil && (status == http.StatusUnauthorized || status == http.StatusBadRequest):
			log.Info("Connection test failed, failing", "status", status)
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    http.StatusForbidden,
					Message: "Invalid credentials",
				},
			}
		case err != nil:
			log.Info("Connection test failed, yet passing", "status", status, "error", err.Error())
		default:
			log.Info("Test succeeded, passing")
		}
		return webhookutils.ToAdmissionResponseAllow()
	} else { // should never happen because of the validating webhook configuration
		log.Info("Secret is not set with 'createdForProviderType', passing")
		return webhookutils.ToAdmissionResponseAllow()
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
