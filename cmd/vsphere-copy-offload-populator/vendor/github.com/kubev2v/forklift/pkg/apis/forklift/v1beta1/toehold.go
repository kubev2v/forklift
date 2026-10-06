package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ToeholdTemplateFinalizer = "forklift/toehold-template"

// LabelToehold marks build pods owned by a ToeholdTemplate (value is the
// template name).
const LabelToehold = "forklift.konveyor.io/toehold"

// ToeholdTemplatePhase is the high-level lifecycle state of a ToeholdTemplate resource.
type ToeholdTemplatePhase string

const (
	ToeholdTemplatePhasePending   ToeholdTemplatePhase = "Pending"
	ToeholdTemplatePhaseRunning   ToeholdTemplatePhase = "Running"
	ToeholdTemplatePhaseSucceeded ToeholdTemplatePhase = "Succeeded"
	ToeholdTemplatePhaseFailed    ToeholdTemplatePhase = "Failed"
)

// ToeholdTemplateStage is the fine-grained pipeline position within the Running phase.
type ToeholdTemplateStage string

const (
	StageEnsurePrerequisites ToeholdTemplateStage = "EnsurePrerequisites"
	StageEnsureTemplate      ToeholdTemplateStage = "EnsureTemplate"
	StageBuildAndUpload      ToeholdTemplateStage = "BuildAndUpload"
	StageToeholdFinished     ToeholdTemplateStage = "Finished"
)

// Condition types set on ToeholdTemplate status.
const (
	ToeholdTemplateFailed   = "ToeholdTemplateFailed"
	ToeholdTemplateUpToDate = "TemplateUpToDate"
)

// ToeholdResources defines CPU and memory for the OVF descriptor.
type ToeholdResources struct {
	// +optional
	// +kubebuilder:default:=2
	CPU int32 `json:"cpu,omitempty"`
	// +optional
	// +kubebuilder:default:=4096
	MemoryMiB int32 `json:"memoryMiB,omitempty"`
}

// ToeholdBaseDisk configures the read-only containerdisk base image.
type ToeholdBaseDisk struct {
	// OCI image embedding the base qcow2 (KubeVirt containerdisk layout under /disk).
	// +optional
	ContainerImage string `json:"containerImage,omitempty"`
	// Optional dockerconfigjson secret for pulling containerImage.
	// +optional
	ImagePullSecret *core.LocalObjectReference `json:"imagePullSecret,omitempty"`
	// emptyDir size limit for the qcow2 overlay workspace. Unset means no limit.
	// +optional
	WorkGiB int64 `json:"workGiB,omitempty"`
}

// ToeholdTemplateSpec defines the desired state of ToeholdTemplate.
type ToeholdTemplateSpec struct {
	// Reference to a vSphere Provider.
	Provider core.ObjectReference `json:"provider"`
	// Base containerdisk image for overlay customization.
	BaseDisk ToeholdBaseDisk `json:"baseDisk"`
	// vCenter template name after OVF import.
	TemplateName string `json:"templateName"`
	// Target vCenter datastore.
	Datastore string `json:"datastore"`
	// vCenter folder path (e.g. /Datacenter/vm).
	Folder string `json:"folder"`
	// Port group for OVF network mapping.
	Network string `json:"network"`
	// Multus NAD for the OVA build pod (same as Plan.spec.transferNetwork).
	// +optional
	TransferNetwork *core.ObjectReference `json:"transferNetwork,omitempty"`
	// Optional node selector for the build pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// OVF hardware descriptor overrides.
	// +optional
	Resources ToeholdResources `json:"resources,omitempty"`
	// Override for the toehold builder container image.
	// Defaults to the controller TOEHOLD_BUILDER_IMAGE setting when empty.
	// +optional
	BuilderImage string `json:"builderImage,omitempty"`
}

// TemplateStatus tracks the vCenter template artifact.
type TemplateStatus struct {
	// True when an existing vCenter template was reused.
	// +optional
	Reused bool `json:"reused,omitempty"`
	// Hash of the base containerdisk image.
	// +optional
	DiskHash string `json:"diskHash,omitempty"`
	// Hash of CPU/memory/network configuration.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`
	// containerImage used for the base disk at build time.
	// +optional
	BaseContainerImage string `json:"baseContainerImage,omitempty"`
	// +optional
	ImportedAt *meta.Time `json:"importedAt,omitempty"`
	// vSphere managed object reference.
	// +optional
	Moref string `json:"moref,omitempty"`
}

// ToeholdTemplateStatus defines the observed state of ToeholdTemplate.
type ToeholdTemplateStatus struct {
	libcnd.Conditions `json:",inline"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
	Phase ToeholdTemplatePhase `json:"phase,omitempty"`
	// +optional
	Stage ToeholdTemplateStage `json:"stage,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// Reference to the build pod when a template was successfully created.
	// +optional
	BuildPod *core.ObjectReference `json:"buildPod,omitempty"`
	// +optional
	Template TemplateStatus `json:"template,omitempty"`
	// +optional
	CompletionTime *meta.Time `json:"completionTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=ttpl
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="STAGE",type=string,JSONPath=".status.stage"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type ToeholdTemplate struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ToeholdTemplateSpec   `json:"spec,omitempty"`
	Status          ToeholdTemplateStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ToeholdTemplateList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []ToeholdTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToeholdTemplate{}, &ToeholdTemplateList{})
}
