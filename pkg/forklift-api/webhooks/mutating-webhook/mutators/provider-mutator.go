package mutators

import (
	"encoding/json"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	admissionv1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	specChanged = mutator.cleanupVddkSettingsIfNeeded() || specChanged
	metadataChanged := mutator.ar.Request.Operation == admissionv1.Create && mutator.setFinalizers()

	var patches []util.PatchOperation
	if specChanged {
		patches = append(patches, util.PatchOperation{
			Op:    "replace",
			Path:  "/spec",
			Value: mutator.provider.Spec,
		})
	}

	if metadataChanged {
		patches = append(patches, util.PatchOperation{
			Op:    "replace",
			Path:  "/metadata",
			Value: mutator.provider.ObjectMeta,
		})
	}

	if len(patches) > 0 {
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
			if mutator.provider.Spec.Settings == nil {
				mutator.provider.Spec.Settings = make(map[string]string)
			}
			mutator.provider.Spec.Settings[api.SDK] = api.VCenter
			providerChanged = true
		}
	}

	return providerChanged
}

func (mutator *ProviderMutator) cleanupVddkSettingsIfNeeded() bool {
	var providerChanged bool

	// Only apply to vSphere providers
	if mutator.provider.Type() != api.VSphere {
		return false
	}

	// Check if VDDK is set
	vddkImage, hasVddk := mutator.provider.Spec.Settings[api.VDDK]
	if hasVddk && vddkImage != "" {
		// VDDK is set, no cleanup needed
		return false
	}

	// VDDK is not set, remove VDDK-specific settings
	if mutator.provider.Spec.Settings == nil {
		return false
	}

	// Remove UseVddkAioOptimization if present
	if _, hasUseVddkAio := mutator.provider.Spec.Settings[api.UseVddkAioOptimization]; hasUseVddkAio {
		delete(mutator.provider.Spec.Settings, api.UseVddkAioOptimization)
		log.Info("Removed UseVddkAioOptimization setting because VDDK is not set")
		providerChanged = true
	}

	// Remove vddkConfig if present
	if _, hasVddkConfig := mutator.provider.Spec.Settings[api.VddkConfig]; hasVddkConfig {
		delete(mutator.provider.Spec.Settings, api.VddkConfig)
		log.Info("Removed VddkConfig setting because VDDK is not set")
		providerChanged = true
	}

	return providerChanged
}

func (mutator *ProviderMutator) setFinalizers() bool {
	var changed bool
	switch mutator.provider.Type() {
	case api.Ova:
		changed = k8sutil.AddFinalizer(&(mutator.provider), api.OvaProviderFinalizer)
	case api.HyperV:
		changed = k8sutil.AddFinalizer(&(mutator.provider), api.HyperVProviderFinalizer)
	}
	return changed
}
