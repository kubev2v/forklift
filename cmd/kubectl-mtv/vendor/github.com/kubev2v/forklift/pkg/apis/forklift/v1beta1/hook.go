package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AnnotationAAPJobTemplateName is an optional Hook metadata annotation for a human-readable
// AAP job template name (UI/CLI). It does not affect execution; only spec.aap.jobTemplateId is used to launch jobs.
const AnnotationAAPJobTemplateName = "forklift.konveyor.io/aap-job-template-name"

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
// Default connection (AAP URL, token Secret name, default HTTP/poll timeouts) is configured on ForkliftController.
// Optional per-hook url, tokenSecret, and timeout override the cluster defaults when set (both url and tokenSecret.name are required to use the hook-local path).
type AAPConfig struct {
	// ID of the AAP job template to execute.
	// +kubebuilder:validation:Required
	JobTemplateID int `json:"jobTemplateId"`
	// Optional per-hook AAP base URL. When set together with tokenSecret, overrides ForkliftController aap_url.
	// +optional
	URL string `json:"url,omitempty"`
	// Optional Secret reference for the AAP API token (key "token"). When set together with url, overrides
	// ForkliftController aap_token_secret_name; the Secret is read from the migration plan namespace.
	// +optional
	TokenSecret *core.ObjectReference `json:"tokenSecret,omitempty" ref:"Secret"`
	// Optional timeout in seconds for polling the AAP job after launch. Overrides defaulting from spec.deadline and ForkliftController aap_timeout when non-zero behavior applies.
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
