package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
type OvirtImageIOPopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec   OvirtImageIOPopulatorSpec   `json:"spec"`
	Status OvirtImageIOPopulatorStatus `json:"status"`
}

type OvirtImageIOPopulatorSpec struct {
	EngineURL        string `json:"engineUrl"`
	EngineSecretName string `json:"engineSecretName"`
	DiskID           string `json:"diskId"`
}

type OvirtImageIOPopulatorStatus struct {
	Progress string `json:"progress"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OvirtImageIOPopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []OvirtImageIOPopulator `json:"items"`
}
