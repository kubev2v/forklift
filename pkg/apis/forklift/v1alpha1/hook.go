package v1alpha1

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
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
