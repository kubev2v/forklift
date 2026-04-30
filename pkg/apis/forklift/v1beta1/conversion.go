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
	// DeepInspection uses vm-migration-detective to inspect the disks
	DeepInspection ConversionType = "DeepInspection"
	// Inspection uses virt-v2v-inspector to inspect the disks (kept only for backward compatibility)
	Inspection ConversionType = "Inspection"
	// InPlace conversion does not use virt-v2v to copy the disks. It only converts the disks in place
	InPlace ConversionType = "InPlace"
	// Remote conversion uses virt-v2v to copy the disks from remote provider to the destination cluster and converts the disks.
	Remote ConversionType = "Remote"
)

// ConversionPhase is the high-level lifecycle state of a Conversion resource
type ConversionPhase string

const (
	PhasePending   ConversionPhase = "Pending"
	PhaseRunning   ConversionPhase = "Running"
	PhaseSucceeded ConversionPhase = "Succeeded"
	PhaseFailed    ConversionPhase = "Failed"
)

// ConversionStage is the fine-grained pipeline position within the Running phase.
type ConversionStage string

const (
	// StageCreatePod is set while the conversion pod is being scheduled.
	StageCreatePod ConversionStage = "CreatingPod"
	// StagePodRunning is set while the conversion pod is actively running.
	StagePodRunning ConversionStage = "PodRunning"
	// StageFinished is always the last stage in every pipeline definition.
	StageFinished ConversionStage = "Finished"
	// StageCreateSnapshot is set while the vSphere snapshot creation task is being submitted.
	StageCreateSnapshot ConversionStage = "CreatingSnapshot"
	// StageWaitForSnapshot is set while polling for the snapshot creation task to complete.
	StageWaitForSnapshot ConversionStage = "WaitingForSnapshot"
	// StageRemoveSnapshot is set while the vSphere snapshot removal task is being submitted.
	StageRemoveSnapshot ConversionStage = "RemovingSnapshot"
	// StageWaitForSnapshotRemoval is set while polling for the snapshot removal task to complete.
	StageWaitForSnapshotRemoval ConversionStage = "WaitingForSnapshotRemoval"
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
	// +kubebuilder:validation:Enum=DeepInspection;Inspection;InPlace;Remote
	Type ConversionType `json:"type"`
	// Reference to the source provider.
	Provider core.ObjectReference `json:"provider" ref:"Provider"`
	// Reference to the destination provider where pods and PVCs live.
	// When empty or pointing to the host provider the local client is used;
	// otherwise a remote k8s client is constructed from the provider URL
	// and its secret.
	// +optional
	DestinationProvider core.ObjectReference `json:"destinationProvider,omitempty" ref:"Provider"`
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
	// VDDK init container image. Required when type is DeepInspection.
	// For other types, empty means no VDDK sidecar.
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

// SnapshotStatus tracks vSphere snapshot tasks and ownership for deep inspection.
type SnapshotStatus struct {
	// Moref is the vSphere managed object reference of the snapshot, used as
	// SNAPSHOT_MOREF in the inspection pod.
	// +optional
	Moref string `json:"moref,omitempty"`
	// Owned is true when the converison controller created the snapshot and is responsible
	// for removing it after the pod exits.
	// +optional
	Owned bool `json:"owned,omitempty"`
	// CreateTaskID is the in-flight vSphere task id for snapshot creation.
	// +optional
	CreateTaskID string `json:"createTaskId,omitempty"`
	// RemoveTaskID is the in-flight vSphere task id for snapshot removal.
	// +optional
	RemoveTaskID string `json:"removeTaskId,omitempty"`
}

// ConversionStatus defines the observed state of Conversion.
type ConversionStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// Used by the update predicate to suppress reconciles triggered by the
	// controller's own status writes.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Phase is the high-level lifecycle state of the conversion.
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
	Phase ConversionPhase `json:"phase,omitempty"`
	// Stage is the current fine-grained pipeline position within the Running phase.
	// Intended for progress observability; the pipeline advances it as work proceeds.
	// +optional
	// +kubebuilder:validation:Enum=PodCreating;PodRunning;CreatingSnapshot;WaitingForSnapshot;RemovingSnapshot;WaitingForSnapshotRemoval;Finished
	Stage ConversionStage `json:"stage,omitempty"`
	// Reference to the managed conversion pod.
	// +optional
	Pod core.ObjectReference `json:"pod,omitempty"`
	// StartTime is when the conversion entered the Running phase.
	// +optional
	StartTime *meta.Time `json:"startTime,omitempty"`
	// CompletionTime is when the conversion reached Succeeded or Failed.
	// +optional
	CompletionTime *meta.Time `json:"completionTime,omitempty"`
	// Snapshot tracks the vSphere snapshot lifecycle for conversions that require
	// a controller-managed snapshot (DeepInspection without a pre-supplied MoRef).
	// +optional
	Snapshot *SnapshotStatus `json:"snapshot,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Conversion is the Schema for the conversions API
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="STAGE",type=string,JSONPath=".status.stage"
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
