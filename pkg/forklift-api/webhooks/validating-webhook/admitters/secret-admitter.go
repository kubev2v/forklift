package admitters

import (
	//	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	//	"fmt"
	//	"net"
	//	"regexp"
	//	"strings"

	//webhookutils "github.com/konveyor/forklift-controller/pkg/util/webhooks"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logging.WithName("admitter")

type SecretAdmitter struct {
}

func (admitter *SecretAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("secret admitter was called")
	raw := ar.Request.Object.Raw
	secret := &core.Secret{}
	err := json.Unmarshal(raw, secret)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			},
		}
	}

	if createdForProviderType, ok := secret.GetLabels()["createdForProviderType"]; ok {
		provider := &api.Provider{}
		providerType := api.ProviderType(createdForProviderType)
		buildProvider(provider, &providerType, secret)

		collector := container.Build(nil, provider, secret)
		if collector == nil {
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("Incorrect 'createdForProviderType' value. Options %s", api.ProviderTypes),
				},
			}
		}

		log.Info("Starting provider connection test")
		status, err := collector.Test()
		if err != nil && (status == http.StatusUnauthorized || status == http.StatusBadRequest) {
			log.Info("Connection test failed, failing")
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    http.StatusForbidden,
					Message: err.Error(),
				},
			}
		} else {
			if err != nil {
				log.Info("Connection test failed, yet passing")
			} else {
				log.Info("Test succeeded, passing")
			}
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
	} else { // should never happen because of the validating webhook configuration
		log.Info("Secret is not set with 'createdForProviderType', passing")
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}
}

func buildProvider(provider *api.Provider, providerType *api.ProviderType, secret *core.Secret) {
	provider.Spec.URL = string(secret.Data["url"])
	provider.Spec.Type = providerType
	provider.Name = secret.Name
	provider.Namespace = secret.Namespace
}
