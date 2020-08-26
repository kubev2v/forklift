/*
Copyright 2019 Red Hat Inc.

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

package v1alpha1

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Pipeline step.
type Step struct {
	// Name.
	Name string `json:"name"`
	// Progress.
	Progress libitr.Progress `json:"progress"`
}

//
// VM errors.
type VMError struct {
	Phase   string   `json:"phase"`
	Reasons []string `json:"reasons"`
}

//
// VM Status
type VMStatus struct {
	// Planned VM.
	Planned PlanVM `json:"planned"`
	// Migration pipeline.
	Pipeline []Step `json:"pipeline"`
	// Phase
	Phase string `json:"phase"`
	// Errors
	Error *VMError `json:"error,omitempty"`
}

//
// MigrationSpec defines the desired state of Migration
type MigrationSpec struct {
	// Reference to the associated Plan.
	Plan core.ObjectReference `json:"plan" ref:"Plan"`
}

//
// MigrationStatus defines the observed state of Migration
type MigrationStatus struct {
	// Conditions.
	libcnd.Conditions
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// VM status
	VMs []VMStatus `json:"vms,omitempty"`
}

//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Migration struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            MigrationSpec   `json:"spec,omitempty"`
	Status          MigrationStatus `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MigrationList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Migration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Migration{}, &MigrationList{})
}
