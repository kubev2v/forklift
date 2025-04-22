package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var VSphereXcopyVolumePopulatorKind = "VSphereXcopyVolumePopulator"
var VSphereXcopyVolumePopulatorResource = "vspherexcopyvolumepopulators"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={vxvp,vxvps}
type VSphereXcopyVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec VSphereXcopyVolumePopulatorSpec `json:"spec"`
	// +optional
	Status VSphereXcopyVolumePopulatorStatus `json:"status"`
}

type VSphereXcopyVolumePopulatorSpec struct {
	// VmdkPath is the full path the vmdk disk. A valid path format is
	// '[$DATASTORE_NAME] $VM_NAME/$DISK_NAME.vmdk'
	VmdkPath string `json:"vmdkPath"`
	// TargetPVC is the kubernetes PVC name that is used as the target
	// The controller will resolve the underlying PV and will copy the data
	// from the vmdk to that target volume
	TargetPVC string `json:"targetPVC"`
	// The secret name with vsphere and storage credentials
	SecretRef string `json:"secretRef"`
	// StorageVendorProduct is the storage vendor the target disk and PVC are connected to
	// Supported values [ontap, ]
	StorageVendorProduct string `json:"storageVendorProduct"`
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
