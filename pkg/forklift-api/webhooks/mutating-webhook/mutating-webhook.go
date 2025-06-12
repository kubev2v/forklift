package mutating_webhook

import (
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("mutating_webhooks")

type mutator interface {
	Mutate(*admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
}

func Serve(resp http.ResponseWriter, req *http.Request, m mutator) {
	review, err := util.GetAdmissionReview(req)
	if err != nil {
		log.Error(err, "mutating Serve error")
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	response := admissionv1.AdmissionReview{
		TypeMeta: v1.TypeMeta{
			APIVersion: review.APIVersion,
			Kind:       "AdmissionReview",
		},
	}
	reviewResponse := m.Mutate(review)
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = review.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	review.Request.Object = runtime.RawExtension{}
	review.Request.OldObject = runtime.RawExtension{}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Error(err, "mutating Serve error, failed to marshal response")
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := resp.Write(responseBytes); err != nil {
		log.Error(err, "mutating Serve error, failed to write response")
		resp.WriteHeader(http.StatusBadRequest)
	}
}
