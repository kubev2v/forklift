package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PortworxXcopyVolumePopulatorKind = "PortworxXcopyVolumePopulator"
var PortworxXcopyVolumePopulatorResource = "portworxxcopyvolumepopulators"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={pxvp,pxvps}
type PortworxXcopyVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec PortworxXcopyVolumePopulatorSpec `json:"spec"`
	// +optional
	Status PortworxXcopyVolumePopulatorStatus `json:"status"`
}

type PortworxXcopyVolumePopulatorSpec struct {
	// SecretName is the secret with storage credentials
	SecretName string `json:"secretName"`
	// SourceNamespace is the namespace of the source PVC
	SourceNamespace string `json:"sourceNamespace"`
	// SourcePvc is the name of the source PVC to copy from
	SourcePvc string `json:"sourcePvc"`
}

type PortworxXcopyVolumePopulatorStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`
	// +optional
	Progress string `json:"progress,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PortworxXcopyVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []PortworxXcopyVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PortworxXcopyVolumePopulator{}, &PortworxXcopyVolumePopulatorList{})
}
