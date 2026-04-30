package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var HyperVVolumePopulatorKind = "HyperVVolumePopulator"
var HyperVVolumePopulatorResource = "hypervvolumepopulators"

// HyperVPopulatorBlockDevicePath is where the raw-block PVC is exposed inside
// the populator pod. The host's /dev is mounted at /host-dev (not /dev) so
// that this VolumeDevice bind mount is not shadowed — mounting hostPath /dev
// over /dev causes kubelet's bind mount to be hidden, making writes land on
// the container's overlay instead of the real block device.
const HyperVPopulatorBlockDevicePath = "/populatorblock"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={hvp,hvps}
type HyperVVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec HyperVVolumePopulatorSpec `json:"spec"`
	// +optional
	Status HyperVVolumePopulatorStatus `json:"status"`
}

type HyperVVolumePopulatorSpec struct {
	// SecretName is the name of the Secret holding Hyper-V provider credentials.
	SecretName string `json:"secretName"`
	// VMID is the unique identifier of the source VM on the Hyper-V host.
	VMID string `json:"vmId"`
	// VMName is the display name of the source VM.
	VMName string `json:"vmName"`
	// DiskIndex is the zero-based index of this disk within the VM's disk list.
	// +kubebuilder:validation:Minimum=0
	DiskIndex int `json:"diskIndex"`
	// DiskPath is the full Windows path to the source VHDX file
	// (e.g. "C:\\VMs\\MyVM\\disk0.vhdx").
	DiskPath string `json:"diskPath"`
	// TargetIQN is the iSCSI Qualified Name of the target created on the Hyper-V host
	// (e.g. "iqn.YYYY-MM.io.forklift:vm-abc123").
	TargetIQN string `json:"targetIQN"`
	// TargetPortal is the iSCSI target portal address in "host:port" format
	// (e.g. "10.0.0.100:3260").
	TargetPortal string `json:"targetPortal"`
	// LunID is the LUN number assigned to this disk within the iSCSI target.
	// +kubebuilder:validation:Minimum=0
	LunID int `json:"lunId"`
	// InitiatorIQN is the IQN the copy pod must use to authenticate via the
	// target's initiator ACL (e.g. "iqn.YYYY-MM.io.forklift:copy-<migrationID>").
	InitiatorIQN string `json:"initiatorIQN"`
}

type HyperVVolumePopulatorStatus struct {
	// Phase describes the current lifecycle phase of the populator.
	// +optional
	Phase string `json:"phase,omitempty"`
	// Progress is a human-readable percentage string (e.g. "45%").
	// +optional
	Progress string `json:"progress,omitempty"`
	// BytesTransferred is the number of bytes written to the destination so far.
	// +optional
	BytesTransferred int64 `json:"bytesTransferred,omitempty"`
	// TotalBytes is the total number of bytes expected for the transfer.
	// +optional
	TotalBytes int64 `json:"totalBytes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HyperVVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []HyperVVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HyperVVolumePopulator{}, &HyperVVolumePopulatorList{})
}
