package mutators

import (
	"context"
	"encoding/json"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	"github.com/kubev2v/forklift/pkg/lib/aap"
	admissionv1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HookMutator struct {
	Client client.Client
}

func (mutator *HookMutator) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	var hook api.Hook
	if err := json.Unmarshal(ar.Request.Object.Raw, &hook); err != nil {
		log.Error(err, "mutating webhook error, failed to unmarshal hook")
		return util.ToAdmissionResponseError(err)
	}
	if hook.Spec.AAP == nil {
		return util.ToAdmissionResponseAllow()
	}
	if name := strings.TrimSpace(hook.Annotations[api.AnnotationAAPJobTemplateName]); name != "" {
		return util.ToAdmissionResponseAllow()
	}

	cl, err := aap.NewClientForHook(context.TODO(), mutator.Client, &hook)
	if err != nil {
		log.V(1).Info("AAP job template name lookup skipped", "error", err.Error())
		return util.ToAdmissionResponseAllow()
	}
	tmpl, err := cl.GetJobTemplate(context.TODO(), hook.Spec.AAP.JobTemplateID)
	if err != nil {
		log.V(1).Info(
			"AAP job template name lookup skipped",
			"error", err.Error(),
			"jobTemplateId", hook.Spec.AAP.JobTemplateID,
		)
		return util.ToAdmissionResponseAllow()
	}

	if hook.Annotations == nil {
		hook.Annotations = map[string]string{}
	}
	hook.Annotations[api.AnnotationAAPJobTemplateName] = strings.TrimSpace(tmpl.Name)

	patchBytes, err := util.GeneratePatchPayload(util.PatchOperation{
		Op:    "replace",
		Path:  "/metadata",
		Value: hook.ObjectMeta,
	})
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
}
