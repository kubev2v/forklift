package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var VSphereNfsCopyVolumePopulatorKind = "VSphereNfsCopyVolumePopulator"
var VSphereNfsCopyVolumePopulatorResource = "vspherenfscopyvolumepopulators"

// IsVSphereCopyOffloadKind determines if the kind is one of the vSphere copy-offload populators
func IsVSphereCopyOffloadKind(kind string) bool {
	return kind == VSphereXcopyVolumePopulatorKind ||
		kind == VSphereNfsCopyVolumePopulatorKind
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={vnvp,vnvps}
type VSphereNfsCopyVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec VSphereNfsCopyVolumePopulatorSpec `json:"spec"`
	// +optional
	Status VSphereNfsCopyVolumePopulatorStatus `json:"status"`
}

type VSphereNfsCopyVolumePopulatorSpec struct {
	// VmId is the VM object id in vSphere
	VmId string `json:"vmId"`
	// VmdkPath is the full path the vmdk disk. A valid path format is
	// '[$DATASTORE_NAME] $VM_HOME/$DISK_NAME.vmdk'
	VmdkPath string `json:"vmdkPath"`
	// The secret name with vsphere and storage credentials
	SecretName string `json:"secretName"`
	// StorageVendorProduct is the storage vendor serving both the source NFS
	// datastore and the destination export.
	// Supported values [pureFlashArray]
	StorageVendorProduct string `json:"storageVendorProduct"`
}

type VSphereNfsCopyVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress"`
	// +optional
	CopyDurationSeconds string `json:"copyDurationSeconds,omitempty"`
	// +optional
	Result string `json:"result,omitempty"`
	// +optional
	StorageVendor string `json:"storageVendor,omitempty"`
	// +optional
	CloneMethod string `json:"cloneMethod,omitempty"`
	// +optional
	StorageProtocol string `json:"storageProtocol,omitempty"`
	// +optional
	ProvisionedBytes string `json:"provisionedBytes,omitempty"`
	// +optional
	AllocatedBytes string `json:"allocatedBytes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VSphereNfsCopyVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []VSphereNfsCopyVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VSphereNfsCopyVolumePopulator{}, &VSphereNfsCopyVolumePopulatorList{})
}
