package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OVAProviderServerSpec struct {
	// Reference to a Provider resource.
	// +kubebuilder:validation:XValidation:message="spec.provider is immutable",rule="oldSelf == null || oldSelf.name == '' || self == oldSelf"
	Provider v1.ObjectReference `json:"provider"`
}

type OVAProviderServerStatus struct {
	// Current life cycle phase of the OVA server.
	// +optional
	Phase string `json:"phase,omitempty"`
	// Reference to Service resource
	// +optional
	Service *v1.ObjectReference `json:"service,omitempty"`
	// Conditions.
	libcnd.Conditions `json:",inline"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type OVAProviderServer struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            OVAProviderServerSpec   `json:"spec,omitempty"`
	Status          OVAProviderServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OVAProviderServerList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []OVAProviderServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OVAProviderServer{}, &OVAProviderServerList{})
}
