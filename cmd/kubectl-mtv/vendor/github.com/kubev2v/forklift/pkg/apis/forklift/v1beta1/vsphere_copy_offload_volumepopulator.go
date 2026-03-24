package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var VSphereCopyOffloadVolumePopulatorKind = "VSphereCopyOffloadVolumePopulator"
var VSphereCopyOffloadVolumePopulatorResource = "vspherecopyoffloadvolumepopulators"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={vcovp,vcovps}
type VSphereCopyOffloadVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec VSphereCopyOffloadVolumePopulatorSpec `json:"spec"`
	// +optional
	Status VSphereCopyOffloadVolumePopulatorStatus `json:"status"`
}

type VSphereCopyOffloadVolumePopulatorSpec struct {
	// VmId is the VM object id in vSphere
	VmId string `json:"vmId"`
	// VmdkPath is the full path the vmdk disk. A valid path format is
	// '[$DATASTORE_NAME] $VM_HOME/$DISK_NAME.vmdk'
	VmdkPath string `json:"vmdkPath"`
	// The secret name with vsphere and storage credentials
	SecretName string `json:"secretName"`
	// StorageVendorProduct is the storage vendor the target disk and PVC are connected to
	// Supported values [vantara, ontap, primera3par]
	StorageVendorProduct string `json:"storageVendorProduct"`
}

type VSphereCopyOffloadVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VSphereCopyOffloadVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []VSphereCopyOffloadVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VSphereCopyOffloadVolumePopulator{}, &VSphereCopyOffloadVolumePopulatorList{})
}
