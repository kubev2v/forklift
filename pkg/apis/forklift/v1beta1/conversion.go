package v1beta1

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConversionType defines the type of conversion to perform.
type ConversionType string

const (
	Inspection ConversionType = "Inspection"
	InPlace    ConversionType = "InPlace"
	Remote     ConversionType = "Remote"
)

// ConversionPhase represents the lifecycle phase of a Conversion resource.
type ConversionPhase string

const (
	PhasePending   ConversionPhase = "Pending"
	PhaseCreating  ConversionPhase = "CreatingPod"
	PhaseRunning   ConversionPhase = "Running"
	PhaseSucceeded ConversionPhase = "Succeeded"
	PhaseFailed    ConversionPhase = "Failed"
)

// DiskRef references a PVC to be used as a disk in the conversion process.
// When created from a Plan, MountPath or DevicePath is populated so the
// Conversion controller can reconstruct pod volumes without re-deriving paths.
type DiskRef struct {
	// Name of the PVC or disk path.
	Name string `json:"name"`
	// Namespace of the PVC.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// VolumeMode indicates whether the disk is block or filesystem.
	// +optional
	VolumeMode *core.PersistentVolumeMode `json:"volumeMode,omitempty"`
	// Filesystem mount path inside the conversion pod (e.g. /mnt/disks/disk0).
	// Mutually exclusive with DevicePath.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
	// Block device path inside the conversion pod (e.g. /dev/block0).
	// Mutually exclusive with MountPath.
	// +optional
	DevicePath string `json:"devicePath,omitempty"`
}

// Connection holds source connection details for the conversion pod.
// Provider-specific values such as libvirtURL and fingerprint are
// expected to be included in the referenced Secret and are injected
// into the pod as V2V_-prefixed environment variables automatically.
type Connection struct {
	// Secret containing virt-v2v credentials and connection parameters.
	Secret core.ObjectReference `json:"secret" ref:"Secret"`
}

// PodSettings groups pod-level overrides for the conversion pod.
type PodSettings struct {
	// Pre-resolved transfer network annotations.
	// +optional
	TransferNetworkAnnotations map[string]string `json:"transferNetworkAnnotations,omitempty"`
	// Labels to add to the conversion pod.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations to add to the conversion pod.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// Node selector constraints for the conversion pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// ServiceAccount for the conversion pod.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Pod affinity/anti-affinity rules.
	// +optional
	Affinity *core.Affinity `json:"affinity,omitempty"`
	// GenerateName prefix for the managed pod.
	// +optional
	GenerateName string `json:"generateName,omitempty"`
}

// ConversionSpec defines the desired state of Conversion.
type ConversionSpec struct {
	// Type of conversion.
	// +kubebuilder:validation:Enum=Inspection;InPlace;Remote
	Type ConversionType `json:"type"`
	// Reference to the provider.
	Provider core.ObjectReference `json:"provider" ref:"Provider"`
	// Reference to the source VM.
	VM ref.Ref `json:"vm"`
	// Disks to be converted or inspected.
	// For InPlace/Remote: populated from PVCs (namespaced name + volume mode).
	// For Inspection: populated with disk paths from the source inventory.
	// +optional
	Disks []DiskRef `json:"disks,omitempty"`
	// Source connection details including the virt-v2v credentials secret.
	Connection Connection `json:"connection"`
	// Disk decryption LUKS keys.
	// +optional
	LUKS core.ObjectReference `json:"luks,omitempty" ref:"Secret"`
	// Container image for the virt-v2v pod. When empty the controller
	// falls back to the global default from settings.
	// +optional
	Image string `json:"image,omitempty"`
	// Namespace where conversion pods will be created.
	// Defaults to the Conversion CR's own namespace.
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// XfsCompatibility selects the XFS-compatible virt-v2v image variant.
	// +optional
	// +kubebuilder:default:=false
	XfsCompatibility bool `json:"xfsCompatibility,omitempty"`
	// Freeform settings passed to the conversion process as environment variables.
	// +optional
	Settings map[string]string `json:"settings,omitempty"`
	// VDDK init container image. Empty means no VDDK sidecar.
	// +optional
	VDDKImage string `json:"vddkImage,omitempty"`
	// Whether the pod needs a KVM device and kubevirt.io/schedulable node selector.
	// +optional
	// +kubebuilder:default:=true
	RequestKVM bool `json:"requestKVM,omitempty"`
	// Sets LOCAL_MIGRATION env var in the conversion pod.
	// +optional
	LocalMigration bool `json:"localMigration,omitempty"`
	// Whether to add UDN open-default-ports annotation.
	// +optional
	UDN bool `json:"udn,omitempty"`
	// Pod-level overrides for the conversion pod.
	// +optional
	PodSettings PodSettings `json:"podSettings,omitempty"`
}

// ConversionStatus defines the observed state of Conversion.
type ConversionStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Phase of the conversion lifecycle.
	// +optional
	// +kubebuilder:validation:Enum=Pending;CreatingPod;Running;Succeeded;Failed
	Phase ConversionPhase `json:"phase,omitempty"`
	// Reference to the managed virt-v2v pod.
	// +optional
	Pod core.ObjectReference `json:"pod,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Conversion is the Schema for the conversions API
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Conversion struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ConversionSpec   `json:"spec,omitempty"`
	Status          ConversionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConversionList contains a list of Conversion
type ConversionList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Conversion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Conversion{}, &ConversionList{})
}
