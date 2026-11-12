package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HyperVProviderServerSpec defines the desired state of HyperVProviderServer.
type HyperVProviderServerSpec struct {
	// Provider reference.
	Provider core.ObjectReference `json:"provider"`
}

// HyperVProviderServerStatus defines the observed state of HyperVProviderServer.
type HyperVProviderServerStatus struct {
	// Phase of the HyperV provider server.
	Phase string `json:"phase,omitempty"`
	// Service reference.
	Service *core.ObjectReference `json:"service,omitempty"`
	// Conditions.
	libcnd.Conditions `json:",inline"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".status.service.name"
type HyperVProviderServer struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            HyperVProviderServerSpec   `json:"spec,omitempty"`
	Status          HyperVProviderServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HyperVProviderServerList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []HyperVProviderServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HyperVProviderServer{}, &HyperVProviderServerList{})
}
