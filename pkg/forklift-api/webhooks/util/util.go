package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func GetAdmissionReview(r *http.Request) (*admissionv1.AdmissionReview, error) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("contentType=%s, expect application/json", contentType)
	}

	ar := &admissionv1.AdmissionReview{}
	err := json.Unmarshal(body, ar)
	return ar, err
}

func GetProviderFromAdmissionReview(ar *admissionv1.AdmissionReview) (new *api.Provider, old *api.Provider, err error) {

	/*if !ValidateRequestResource(ar.Request.Resource, webhooks.VirtualMachineInstanceGroupVersionResource.Group, webhooks.VirtualMachineInstanceGroupVersionResource.Resource) {
		return nil, nil, fmt.Errorf("expect resource to be '%s'", webhooks.VirtualMachineInstanceGroupVersionResource.Resource)
	}*/

	raw := ar.Request.Object.Raw
	newProvider := api.Provider{}

	err = json.Unmarshal(raw, &newProvider)
	if err != nil {
		return nil, nil, err
	}

	/*if ar.Request.Operation == admissionv1.Update {
		raw := ar.Request.OldObject.Raw
		oldVMI := v12.VirtualMachineInstance{}
		err = json.Unmarshal(raw, &oldVMI)
		if err != nil {
			return nil, nil, err
		}
		return &newVMI, &oldVMI, nil
	}*/

	return &newProvider, nil, nil
}

// ToAdmissionResponseError
func ToAdmissionResponseError(err error) *admissionv1.AdmissionResponse {
	//log.Log.Reason(err).Error("admission generic error")

	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &v1.Status{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		},
	}
}

func GeneratePatchPayload(patches ...PatchOperation) ([]byte, error) {
	if len(patches) == 0 {
		return nil, fmt.Errorf("list of patches is empty")
	}

	payloadBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, err
	}

	return payloadBytes, nil
}

// ToAdmissionResponseAllow returns allowed response
func ToAdmissionResponseAllow() *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}
