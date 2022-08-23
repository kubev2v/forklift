package admitters

import (
	//	"encoding/base64"
	"encoding/json"
	"net/http"
	"fmt"

	//	"fmt"
	//	"net"
	//	"regexp"
	//	"strings"

	//webhookutils "github.com/konveyor/forklift-controller/pkg/util/webhooks"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretAdmitter struct {
}

func (admitter *SecretAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	/*	if resp := webhookutils.ValidateSchema(v1.VirtualMachineInstanceGroupVersionKind, ar.Request.Object.Raw); resp != nil {
			return resp
		}
		1GaccountName := ar.Request.UserInfo.Username
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

	log.Info("secret admitter was called")
	raw := ar.Request.Object.Raw
	secret := &core.Secret{}
	err := json.Unmarshal(raw, secret)
	if err != nil {
//		reviewResponse := admissionv1.AdmissionResponse{}
//		reviewResponse.Allowed = false
//		return &reviewResponse
		return &admissionv1.AdmissionResponse{
                        Allowed: false,
                        Result: &metav1.Status{
                                Code:    http.StatusBadRequest,
                                Message: err.Error(),
                        },
                }
	}
	createdForProviderType, ok := secret.GetLabels()["createdForProviderType"]
	// TODO: need to change this, secret without labels should be allowed
	if !ok {
		return &admissionv1.AdmissionResponse{
                        Allowed: false,
                        Result: &metav1.Status{
                                Code:    http.StatusBadRequest,
                                Message: "The label 'createdForProviderType' is not set on secret",
                        },
                }
	}
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
	err = collector.Test()
	if err != nil {
		return &admissionv1.AdmissionResponse{
                        Allowed: false,
                        Result: &metav1.Status{
                                Code:    http.StatusForbidden,
                                Message: err.Error(),
                        },
                }
	}
	log.Info("Provider connection test passed")
	return &admissionv1.AdmissionResponse{
                Allowed: true,
        }
}

func buildProvider(provider *api.Provider, providerType *api.ProviderType, secret *core.Secret) {
	provider.Spec.URL = string(secret.Data["url"])
	provider.Spec.Type = providerType
	provider.Name = secret.Name
	provider.Namespace = secret.Namespace
}
