package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicProviderSpec defines the desired state of a dynamic provider.
// Defines a provider type and default configuration for server instances.
type DynamicProviderSpec struct {
	// Type identifier for this provider (e.g., "ova").
	// Must be unique across all DynamicProvider resources.
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// DisplayName is a human-readable name for this provider type.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description of this provider type and its purpose.
	// +optional
	Description string `json:"description,omitempty"`

	// Container image for the provider server.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// ImagePullPolicy for the provider container.
	// +optional
	// +kubebuilder:default=Always
	ImagePullPolicy *core.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets for pulling the provider image.
	// +optional
	ImagePullSecrets []core.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Default service port for the provider server.
	// +optional
	// +kubebuilder:default=8080
	Port *int32 `json:"port,omitempty"`

	// Default inventory refresh interval in seconds.
	// Set to 0 to disable automatic polling.
	// +optional
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=0
	RefreshInterval *int32 `json:"refreshInterval,omitempty"`

	// Feature flags defining the provider's capabilities.
	// +optional
	Features *ProviderFeatures `json:"features,omitempty"`

	// Storage volumes to create for provider servers.
	// Each storage definition results in a PVC being created and mounted in the server pod.
	// +optional
	Storages []StorageSpec `json:"storages,omitempty"`

	// Default environment variables for provider server containers.
	// +optional
	Env []core.EnvVar `json:"env,omitempty"`

	// Default resource requirements for provider server containers.
	// +optional
	Resources *core.ResourceRequirements `json:"resources,omitempty"`
}

// StorageSpec defines a storage volume to create for a provider server.
type StorageSpec struct {
	// Name of the storage volume. Must be unique within the provider.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Size of the storage volume.
	// +kubebuilder:validation:Required
	Size string `json:"size"`

	// Mount path in the container.
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`

	// Storage class name for the volume.
	// If not specified, the default storage class will be used.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// Access mode for the volume.
	// +optional
	// +kubebuilder:default=ReadWriteOnce
	AccessMode *core.PersistentVolumeAccessMode `json:"accessMode,omitempty"`

	// Volume mode for the volume.
	// +optional
	// +kubebuilder:default=Filesystem
	VolumeMode *core.PersistentVolumeMode `json:"volumeMode,omitempty"`
}

// DynamicProviderStatus defines the observed state of a dynamic provider.
type DynamicProviderStatus struct {
	// Current phase of the provider registration.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Number of DynamicProviderServer instances using this provider.
	// +optional
	ServerCount int32 `json:"serverCount,omitempty"`

	// List of DynamicProviderServer instances using this provider.
	// +optional
	Servers []core.ObjectReference `json:"servers,omitempty"`

	// ObservedGeneration is the generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the provider's state.
	libcnd.Conditions `json:",inline"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="IMAGE",type=string,JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="SERVERS",type=integer,JSONPath=".status.serverCount"
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=dynamicproviders,shortName=dp,scope=Cluster
type DynamicProvider struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            DynamicProviderSpec   `json:"spec,omitempty"`
	Status          DynamicProviderStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DynamicProviderList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []DynamicProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicProvider{}, &DynamicProviderList{})
}
