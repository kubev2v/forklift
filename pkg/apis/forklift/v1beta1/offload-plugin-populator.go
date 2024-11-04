package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var OffloadPluginVolumePopulatorKind = "OffloadPluginVolumePopulator"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={opvp,opvps}
type OffloadPluginVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec OffloadPluginVolumePopulatorSpec `json:"spec"`
	// +optional
	Status OffloadPluginVolumePopulatorStatus `json:"status"`
}

type OffloadPluginVolumePopulatorSpec struct {
	Image      string `json:"image"`
	File       string `json:"file,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

type OffloadPluginVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OffloadPluginVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []OffloadPluginVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OffloadPluginVolumePopulator{}, &OffloadPluginVolumePopulatorList{})
}
