/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualizationValidationProfile selects the validation input scope.
type VirtualizationValidationProfile string

const (
	// VirtualizationValidationProviderProfile runs generic, advisory target checks.
	VirtualizationValidationProviderProfile VirtualizationValidationProfile = "Provider"
	// VirtualizationValidationPlanProfile is reserved for future Plan-aware checks.
	VirtualizationValidationPlanProfile VirtualizationValidationProfile = "Plan"
)

// VirtualizationValidationPhase is the lifecycle phase of a validation run.
type VirtualizationValidationPhase string

const (
	VirtualizationValidationPending   VirtualizationValidationPhase = "Pending"
	VirtualizationValidationRunning   VirtualizationValidationPhase = "Running"
	VirtualizationValidationSucceeded VirtualizationValidationPhase = "Succeeded"
	VirtualizationValidationFailed    VirtualizationValidationPhase = "Failed"
)

// VirtualizationValidationSpec defines a hub-local validation request.
type VirtualizationValidationSpec struct {
	// ProviderRef is the OpenShift target Provider to validate.
	// +kubebuilder:validation:Required
	ProviderRef core.ObjectReference `json:"providerRef" ref:"Provider"`
	// Profile selects the validation input scope. The initial implementation supports Provider only.
	// +kubebuilder:validation:Enum=Provider
	// +kubebuilder:default=Provider
	Profile VirtualizationValidationProfile `json:"profile,omitempty"`
	// Checks selects IDs from the trusted validator catalog.
	// +optional
	Checks []string `json:"checks,omitempty"`
	// ValidationNamespace is the dedicated remote namespace for workload checks.
	// It must not contain user workloads because the validator cleans up test
	// resources in this namespace.
	// +optional
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=63
	ValidationNamespace string `json:"validationNamespace,omitempty"`
	// RunNonce requests a new run without otherwise changing inputs.
	// +optional
	RunNonce string `json:"runNonce,omitempty"`
}

// VirtualizationValidationSummary is a bounded projection of the current report.
type VirtualizationValidationSummary struct {
	Total   int `json:"total,omitempty"`
	Pending int `json:"pending,omitempty"`
	Passed  int `json:"passed,omitempty"`
	Failed  int `json:"failed,omitempty"`
	Skipped int `json:"skipped,omitempty"`
}

// VirtualizationValidationCheckResult is a bounded projection of one CTRF test.
type VirtualizationValidationCheckResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Duration int    `json:"duration,omitempty"`
}

// VirtualizationValidationStatus defines observed validation state.
type VirtualizationValidationStatus struct {
	libcnd.Conditions `json:",inline"`
	// Phase is the lifecycle phase of the effective validation run.
	// +optional
	Phase VirtualizationValidationPhase `json:"phase,omitempty"`
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ObservedInputHash identifies the inputs used for the current run.
	// +optional
	ObservedInputHash string `json:"observedInputHash,omitempty"`
	// JobRef identifies the hub-local validator Job.
	// +optional
	JobRef *core.ObjectReference `json:"jobRef,omitempty"`
	// ResultRef identifies the hub-local result ConfigMap.
	// +optional
	ResultRef *core.ObjectReference `json:"resultRef,omitempty"`
	// Image is the digest-pinned validator image used for this run.
	// +optional
	Image string `json:"image,omitempty"`
	// Summary is the latest bounded report summary.
	// +optional
	Summary VirtualizationValidationSummary `json:"summary,omitempty"`
	// CheckResults contains the latest outcomes for selected trusted checks.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=32
	CheckResults []VirtualizationValidationCheckResult `json:"checkResults,omitempty"`
	// StartedAt is when the current run was created.
	// +optional
	StartedAt *meta.Time `json:"startedAt,omitempty"`
	// CompletedAt is when the current run reached a terminal state.
	// +optional
	CompletedAt *meta.Time `json:"completedAt,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".spec.providerRef.name"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type VirtualizationValidation struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            VirtualizationValidationSpec   `json:"spec,omitempty"`
	Status          VirtualizationValidationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VirtualizationValidationList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []VirtualizationValidation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualizationValidation{}, &VirtualizationValidationList{})
}
