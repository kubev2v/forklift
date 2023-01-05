package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
type OvirtVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec   OvirtVolumePopulatorSpec   `json:"spec"`
	Status OvirtVolumePopulatorStatus `json:"status"`
}

type OvirtVolumePopulatorSpec struct {
	EngineURL        string `json:"engineUrl"`
	EngineSecretName string `json:"engineSecretName"`
	DiskID           string `json:"diskId"`
}

type OvirtVolumePopulatorStatus struct {
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
