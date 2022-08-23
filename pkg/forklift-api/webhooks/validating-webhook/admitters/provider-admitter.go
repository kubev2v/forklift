package admitters

import (
	//	"encoding/base64"
	"encoding/json"
	"net/http"

	//	"fmt"
	//	"net"
	//	"regexp"
	//	"strings"

	//webhookutils "github.com/konveyor/forklift-controller/pkg/util/webhooks"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logging.WithName("admitter")

type ProviderAdmitter struct {
}

func (admitter *ProviderAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	/*	if resp := webhookutils.ValidateSchema(v1.VirtualMachineInstanceGroupVersionKind, ar.Request.Object.Raw); resp != nil {
			return resp
		}
		accountName := ar.Request.UserInfo.Username
		provider, _, err := webhookutils.GetProviderFromAdmissionReview(ar)
		if err != nil {
			return webhookutils.ToAdmissionResponseError(err)
		}
		causes := ValidateVirtualMachineInstanceSpec(k8sfield.NewPath("spec"), &vmi.Spec, admitter.ClusterConfig)
		// We only want to validate that volumes are mapped to disks or filesystems during VMI admittance, thus this logic is seperated from the above call that is shared with the VM admitter.
		causes = append(causes, validateVirtualMachineInstanceSpecVolumeDisks(k8sfield.NewPath("spec"), &vmi.Spec)...)
		causes = append(causes, ValidateVirtualMachineInstanceMandatoryFields(k8sfield.NewPath("spec"), &vmi.Spec)...)
		causes = append(causes, ValidateVirtualMachineInstanceMetadata(k8sfield.NewPath("metadata"), &vmi.ObjectMeta, admitter.ClusterConfig, accountName)...)
		// In a future, yet undecided, release either libvirt or QEMU are going to check the hyperv dependencies, so we can get rid of this code.
		causes = append(causes, webhooks.ValidateVirtualMachineInstanceHypervFeatureDependencies(k8sfield.NewPath("spec"), &vmi.Spec)...)
		if webhooks.IsARM64() {
			// Check if there is any unsupported setting if the arch is Arm64
			causes = append(causes, webhooks.ValidateVirtualMachineInstanceArm64Setting(k8sfield.NewPath("spec"), &vmi.Spec)...)
		}
		if len(causes) > 0 {
			return webhookutils.ToAdmissionResponse(causes)
		}*/

	log.Info("admitter was called")
	raw := ar.Request.Object.Raw
	newProvider := api.Provider{}
	err := json.Unmarshal(raw, &newProvider)
	if err != nil {
		reviewResponse := admissionv1.AdmissionResponse{}
		reviewResponse.Allowed = false
		return &reviewResponse
	}
	if newProvider.Name == "arik" {
		reviewResponse := admissionv1.AdmissionResponse{}
		reviewResponse.Allowed = true
		return &reviewResponse
	}
	return &admissionv1.AdmissionResponse{
		Result: &v1.Status{
			Message: "michal is that  you?",
			Code:    http.StatusBadRequest,
		},
	}
}
