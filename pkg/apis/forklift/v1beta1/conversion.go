package v1beta1

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConversionType defines the type of conversion to perform.
type ConversionType string

const (
	Inspection ConversionType = "Inspection"
	InPlace    ConversionType = "InPlace"
	Cold       ConversionType = "Cold"
)

// ConversionSpec defines the desired state of Conversion.
type ConversionSpec struct {
	// Type of conversion.
	// +kubebuilder:validation:Enum=Inspection;InPlace;Cold
	Type ConversionType `json:"type"`
	// Reference to the provider.
	Provider core.ObjectReference `json:"provider" ref:"Provider"`
	// Reference to the source VM.
	VM ref.Ref `json:"vm"`
	// Disk decryption LUKS keys.
	// +optional
	LUKS core.ObjectReference `json:"luks,omitempty" ref:"Secret"`
	// Freeform settings passed to the conversion process.
	// +optional
	Settings map[string]string `json:"settings,omitempty"`
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
