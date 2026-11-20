package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var NutanixNfsVolumePopulatorKind = "NutanixNfsVolumePopulator"
var NutanixNfsVolumePopulatorResource = "nutanixnfsvolumepopulators"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={nnvp,nnvps}
//
// NutanixNfsVolumePopulator copies a Nutanix VM disk into a target PVC by
// reading the disk file directly from the owning Prism Element cluster's
// storage container over NFS, bypassing the Prism REST Image Service (and its
// HTTP 429 rate limiting).
type NutanixNfsVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec NutanixNfsVolumePopulatorSpec `json:"spec"`
	// +optional
	Status NutanixNfsVolumePopulatorStatus `json:"status"`
}

type NutanixNfsVolumePopulatorSpec struct {
	// ServerIP is the NFS endpoint to mount. This is the owning Prism Element
	// cluster's external/virtual IP (Cluster.ExternalIP), NOT Prism Central.
	ServerIP string `json:"serverIP"`
	// Export is the storage container name exported over NFS (e.g.
	// "SelfServiceContainer").
	Export string `json:"export"`
	// SourcePath is the path to the disk file within the export, e.g.
	// ".acropolis/vmdisk/<vmdisk-uuid>" for a direct vmdisk read, or the
	// catalog image's file path when using the clone-to-image route.
	SourcePath string `json:"sourcePath"`
	// DiskSizeBytes is the provisioned (virtual) size of the source disk.
	DiskSizeBytes int64 `json:"diskSizeBytes"`
	// SecretName optionally references credentials / CA / mount options for
	// the transfer. NFSv3 is unauthenticated, so this is reserved for future
	// use (e.g. mount tuning or a TLS-fronted endpoint).
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

type NutanixNfsVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress,omitempty"`
	// +optional
	TransferredBytes int64 `json:"transferredBytes,omitempty"`
	// +optional
	Result string `json:"result,omitempty"`
	// +optional
	CopyDurationSeconds string `json:"copyDurationSeconds,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NutanixNfsVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []NutanixNfsVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NutanixNfsVolumePopulator{}, &NutanixNfsVolumePopulatorList{})
}
