package admitters

import (
	"encoding/json"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	admissionv1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderAdmitter struct {
	Client   client.Client
	provider api.Provider
}

func (admitter *ProviderAdmitter) validateSdkEndpointType() error {
	endpoint, ok := admitter.provider.Spec.Settings[api.SDK]
	if ok && admitter.provider.Type() == api.VSphere && endpoint != api.VCenter && endpoint != api.ESXI {
		return liberr.New("vSphere provider is set with an invalid SDK endpoint type", "endpoint", endpoint)
	}
	return nil
}

// validateDatacenter rejects a vSphere provider that sets the datacenter
// setting while its SDK endpoint is esxi.
func (admitter *ProviderAdmitter) validateDatacenter() error {
	datacenter, ok := admitter.provider.Spec.Settings[api.Datacenter]
	if ok && datacenter != "" && admitter.provider.Type() == api.VSphere && admitter.provider.Spec.Settings[api.SDK] == api.ESXI {
		return liberr.New(
			"vSphere provider with an ESXi SDK endpoint cannot restrict the inventory to a datacenter; "+
				"the datacenter setting requires a vCenter SDK endpoint",
			"datacenter", datacenter,
			"endpoint", api.ESXI)
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

	if err := admitter.validateDatacenter(); err != nil {
		return util.ToAdmissionResponseError(err)
	}

	return util.ToAdmissionResponseAllow()
}
