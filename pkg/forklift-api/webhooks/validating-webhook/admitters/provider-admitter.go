package admitters

import (
	"encoding/json"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/settings"
	admissionv1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderAdmitter struct {
	Client   client.Client
	provider api.Provider
}

func (admitter *ProviderAdmitter) validateVddkImage() error {
	image := settings.GetVDDKImage(admitter.provider.Spec.Settings)
	if image != "" {
		if image == "" {
			err := liberr.New("The specified VDDK init image name is empty")
			log.Error(err, "The specified VDDK init image cannot be empty, failing",
				"provider", admitter.provider.Name,
				"namespace", admitter.provider.Namespace)
			return err
		}
	}
	return nil
}

func (admitter *ProviderAdmitter) validateSdkEndpointType() error {
	endpoint, ok := admitter.provider.Spec.Settings[api.SDK]
	if ok && admitter.provider.Type() == api.VSphere && endpoint != api.VCenter && endpoint != api.ESXI {
		return liberr.New("vSphere provider is set with an invalid SDK endpoint type", "endpoint", endpoint)
	}
	return nil
}

func (admitter *ProviderAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Provider admitter was called")
	raw := ar.Request.Object.Raw

	if err := json.Unmarshal(raw, &admitter.provider); err != nil {
		return util.ToAdmissionResponseError(err)
	}

	if err := admitter.validateSdkEndpointType(); err != nil {
		return util.ToAdmissionResponseError(err)
	}

	return util.ToAdmissionResponseAllow()
}
