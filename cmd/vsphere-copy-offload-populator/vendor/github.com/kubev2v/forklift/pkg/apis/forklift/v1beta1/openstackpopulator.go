package v1beta1

import (
	core "k8s.io/api/core/v1"
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
	// The network attachment definition that should be used for disk transfer.
	TransferNetwork *core.ObjectReference `json:"transferNetwork,omitempty"`
}

type OpenstackVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress"`
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
