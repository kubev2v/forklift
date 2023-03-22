package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var OpenstackVolumePopulatorKind = "OpenstackVolumePopulator"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={osvp,osvps}
type OpenstackVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec OpenstackVolumePopulatorSpec `json:"spec"`
	// +optional
	Status OpenstackVolumePopulatorStatus `json:"status"`
}

type OpenstackVolumePopulatorSpec struct {
	IdentityURL string `json:"identityUrl"`
	SecretName  string `json:"secretName"`
	ImageID     string `json:"imageId"`
}

type OpenstackVolumePopulatorStatus struct {
	// +optional
	Transferred string `json:"transferred"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OpenstackVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []OpenstackVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenstackVolumePopulator{}, &OpenstackVolumePopulatorList{})
}
