package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Hook specification.
type HookSpec struct {
	// Service account.
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image to run.
	Image string `json:"image"`
	// A base64 encoded Ansible playbook.
	Playbook string `json:"playbook,omitempty"`
	// Hook deadline in seconds.
	Deadline int64 `json:"deadline,omitempty"`
	// AAP (Ansible Automation Platform) configuration for remote job execution.
	// When specified, the hook will trigger an AAP job template instead of running a local playbook.
	// +optional
	AAP *AAPConfig `json:"aap,omitempty"`
}

// AAPConfig defines configuration for executing hooks via Ansible Automation Platform.
type AAPConfig struct {
	// URL of the AAP instance (e.g., "https://aap.example.com").
	// +kubebuilder:validation:Required
	URL string `json:"url"`
	// ID of the AAP job template to execute.
	// +kubebuilder:validation:Required
	JobTemplateID int `json:"jobTemplateId"`
	// Reference to a Secret containing the AAP API token.
	// The Secret must contain a key named "token" with the Bearer token value.
	// +kubebuilder:validation:Required
	TokenSecret string `json:"tokenSecret"`
	// Timeout for AAP job execution in seconds.
	// If not specified, defaults to the Hook deadline.
	// +optional
	Timeout int64 `json:"timeout,omitempty"`
}

// Hook status.
type HookStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Hook is the Schema for the hooks API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Hook struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            HookSpec   `json:"spec,omitempty"`
	Status          HookStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HookList contains a list of MigHook
type HookList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Hook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Hook{}, &HookList{})
}
