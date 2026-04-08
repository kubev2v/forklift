package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConversionSpec defines the desired state of Conversion.
type ConversionSpec struct {
}

// ConversionStatus defines the observed state of Conversion.
type ConversionStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Conversion is the Schema for the conversions API
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Conversion struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ConversionSpec   `json:"spec,omitempty"`
	Status          ConversionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConversionList contains a list of Conversion
type ConversionList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Conversion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Conversion{}, &ConversionList{})
}
