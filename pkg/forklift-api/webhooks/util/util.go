package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAdmissionReview(r *http.Request) (*admissionv1.AdmissionReview, error) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
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
		Result: &v1.Status{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		},
	}
}
