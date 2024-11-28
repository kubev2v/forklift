package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO go import this from the vsphere populator repo. Perhaps this should not go into
// a separate repo?
var VSphereXcopyVolumePopulatorKind = "VSphereXcopyVolumePopulator"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={ovvp,ovvps}
type VSphereXcopyVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec VSphereXcopyVolumePopulatorSpec `json:"spec"`
	// +optional
	Status VSphereXcopyVolumePopulatorStatus `json:"status"`
}

type VSphereXcopyVolumePopulatorSpec struct {
	VmdkPath  string `json:"vmdkPath"`
	TargetPVC string `json:"targetPVC"`
}

type VSphereXcopyVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VSphereXcopyVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []VSphereXcopyVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VSphereXcopyVolumePopulator{}, &VSphereXcopyVolumePopulatorList{})
}
