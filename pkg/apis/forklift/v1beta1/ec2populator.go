package v1beta1

import (
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Ec2VolumePopulatorKind     = "Ec2VolumePopulator"
	Ec2VolumePopulatorResource = "ec2volumepopulators"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={ec2vp,ec2vps}
type Ec2VolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            Ec2VolumePopulatorSpec   `json:"spec"`
	Status          Ec2VolumePopulatorStatus `json:"status,omitempty"`
}

type Ec2VolumePopulatorSpec struct {
	Region                 string                `json:"region"`                 // Required - AWS region where snapshot exists and volume will be created
	TargetAvailabilityZone string                `json:"targetAvailabilityZone"` // Required - AZ where OpenShift workers are
	SnapshotID             string                `json:"snapshotId"`
	SecretName             string                `json:"secretName"`
	TransferNetwork        *core.ObjectReference `json:"transferNetwork,omitempty"`
}

type Ec2VolumePopulatorStatus struct {
	Progress string `json:"progress,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Ec2VolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Ec2VolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ec2VolumePopulator{}, &Ec2VolumePopulatorList{})
}
