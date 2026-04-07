package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Hook specification.
// Local hooks require spec.image (playbook is optional if the image runs without an injected playbook).
// AAP hooks require spec.aap (image/playbook omitted for execution).
// Whether the spec is valid for execution is enforced by the hook and plan controllers (not by CRD admission rules).
type HookSpec struct {
	// Service account.
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image to run the hook workload (required for local hooks; omit for AAP hooks).
	// +optional
	Image string `json:"image,omitempty"`
	// A base64 encoded Ansible playbook (optional for local hooks; when set, ansible-runner is used).
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
	// Reference to the Secret containing the AAP API token.
	// The Secret must contain a key named "token" with the Bearer token value.
	// The controller reads the Secret only from the migration plan namespace.
	// Namespace must be empty or equal to that plan namespace.
	// +kubebuilder:validation:Required
	TokenSecret core.ObjectReference `json:"tokenSecret" ref:"Secret"`
	// Timeout in seconds to wait for the AAP job after launch (wall-clock polling limit).
	// Default when omitted or 0: first use HookSpec.deadline if it is > 0; otherwise 3600 seconds (1 hour).
	// Any negative value: wait without a wall-clock limit until the job finishes or fails.
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
