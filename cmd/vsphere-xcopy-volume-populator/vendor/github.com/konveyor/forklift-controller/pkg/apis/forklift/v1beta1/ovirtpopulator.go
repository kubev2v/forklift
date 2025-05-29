package v1beta1

import (
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var OvirtVolumePopulatorKind = "OvirtVolumePopulator"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={ovvp,ovvps}
type OvirtVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec OvirtVolumePopulatorSpec `json:"spec"`
	// +optional
	Status OvirtVolumePopulatorStatus `json:"status"`
}

type OvirtVolumePopulatorSpec struct {
	EngineURL        string `json:"engineUrl"`
	EngineSecretName string `json:"engineSecretName"`
	DiskID           string `json:"diskId"`
	// The network attachment definition that should be used for disk transfer.
	TransferNetwork *core.ObjectReference `json:"transferNetwork,omitempty"`
}

type OvirtVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OvirtVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []OvirtVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OvirtVolumePopulator{}, &OvirtVolumePopulatorList{})
}
