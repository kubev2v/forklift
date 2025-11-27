package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicProviderServerSpec defines the desired state of a dynamic provider server instance.
type DynamicProviderServerSpec struct {
	// Reference to the DynamicProvider that defines the provider type.
	// +kubebuilder:validation:Required
	DynamicProviderRef ProviderReference `json:"dynamicProviderRef"`

	// Reference to the Provider CR that this server serves.
	// +kubebuilder:validation:Required
	ProviderRef ProviderReference `json:"providerRef"`

	// Container image for the provider server.
	// Overrides the plugin default if specified.
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy for the provider server container.
	// +optional
	ImagePullPolicy *core.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets for pulling the provider server image.
	// +optional
	ImagePullSecrets []core.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Number of replicas for the server deployment.
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Service port for the provider server.
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Inventory refresh interval in seconds.
	// Set to 0 to disable automatic polling.
	// +optional
	// +kubebuilder:validation:Minimum=0
	RefreshInterval *int32 `json:"refreshInterval,omitempty"`

	// Storage volumes for this provider server.
	// These are dynamically created as PVCs by the controller.
	// Populated from DynamicProvider.spec.storages.
	// +optional
	Storages []StorageSpec `json:"storages,omitempty"`

	// Volumes to mount in the server pod from existing sources.
	// These are NOT created by the controller - they reference existing resources
	// or inline volume definitions (NFS, ConfigMap, Secret, existing PVC, etc.).
	// Populated from Provider.spec.volumes.
	// +optional
	Volumes []ProviderVolume `json:"volumes,omitempty"`

	// Environment variables for the provider server container.
	// +optional
	Env []core.EnvVar `json:"env,omitempty"`

	// Resource requirements for the provider server container.
	// +optional
	Resources *core.ResourceRequirements `json:"resources,omitempty"`

	// Security context for the provider server pod.
	// +optional
	SecurityContext *core.PodSecurityContext `json:"securityContext,omitempty"`

	// Service type for the provider server service.
	// +optional
	// +kubebuilder:default=ClusterIP
	ServiceType *core.ServiceType `json:"serviceType,omitempty"`

	// Node selector for scheduling the provider server pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for scheduling the provider server pod.
	// +optional
	Tolerations []core.Toleration `json:"tolerations,omitempty"`

	// Affinity rules for scheduling the provider server pod.
	// +optional
	Affinity *core.Affinity `json:"affinity,omitempty"`
}

// ProviderReference identifies a Provider or DynamicProvider resource.
type ProviderReference struct {
	// Name of the Provider resource.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Provider resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// DynamicProviderServerStatus defines the observed state of a dynamic provider server instance.
type DynamicProviderServerStatus struct {
	// Current life cycle phase of the provider server.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Reference to the Deployment resource.
	// +optional
	Deployment *core.ObjectReference `json:"deployment,omitempty"`

	// Reference to the Service resource.
	// +optional
	Service *core.ObjectReference `json:"service,omitempty"`

	// References to the PersistentVolumeClaim resources.
	// +optional
	PVCs []core.ObjectReference `json:"pvcs,omitempty"`

	// Deployment ready status.
	// +optional
	DeploymentReady bool `json:"deploymentReady,omitempty"`

	// Number of ready replicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Service URL for accessing the provider server.
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// The provider type this server implements.
	// +optional
	ProviderType string `json:"providerType,omitempty"`

	// ObservedGeneration is the generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the server's state.
	libcnd.Conditions `json:",inline"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="DYNAMIC-PROVIDER",type=string,JSONPath=".spec.dynamicProviderRef.name"
// +kubebuilder:printcolumn:name="PROVIDER",type=string,JSONPath=".spec.providerRef.name"
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=".status.providerType"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.deploymentReady"
// +kubebuilder:printcolumn:name="REPLICAS",type=integer,JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=dynamicproviderservers,shortName=dps
type DynamicProviderServer struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            DynamicProviderServerSpec   `json:"spec,omitempty"`
	Status          DynamicProviderServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DynamicProviderServerList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []DynamicProviderServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicProviderServer{}, &DynamicProviderServerList{})
}
