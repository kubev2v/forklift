package mutators

import (
	"encoding/json"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderMutator struct {
	ar       *admissionv1.AdmissionReview
	provider api.Provider
	Client   client.Client
}

func (mutator *ProviderMutator) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("provider mutator was called")
	mutator.ar = ar
	raw := ar.Request.Object.Raw
	if err := json.Unmarshal(raw, &mutator.provider); err != nil {
		log.Error(err, "mutating webhook error, failed to unmarshel provider")
		return util.ToAdmissionResponseError(err)
	}

	specChanged := mutator.setSdkEndpointIfNeeded()

	if specChanged {
		patches := mutator.patchPayload()
		patchBytes, err := util.GeneratePatchPayload(patches...)
		if err != nil {
			log.Error(err, "mutating webhook error, failed to generate payload for patch request")
			return util.ToAdmissionResponseError(err)
		}

		jsonPatchType := admissionv1.PatchTypeJSONPatch
		return &admissionv1.AdmissionResponse{
			Allowed:   true,
			Patch:     patchBytes,
			PatchType: &jsonPatchType,
		}
	} else {
		return util.ToAdmissionResponseAllow()
	}
}

func (mutator *ProviderMutator) setSdkEndpointIfNeeded() bool {
	var providerChanged bool

	if mutator.provider.Type() == api.VSphere {
		if _, ok := mutator.provider.Spec.Settings[api.SDK]; !ok {
			log.Info("SDK endpoint type was not specified for a vSphere provider, assuming vCenter")
			mutator.provider.Spec.Settings[api.SDK] = api.VCenter
			providerChanged = true
		}
	}

	return providerChanged
}

func (mutator *ProviderMutator) patchPayload() []util.PatchOperation {
	return []util.PatchOperation{{
		Op:    "replace",
		Path:  "/spec",
		Value: mutator.provider.Spec,
	}}
}
