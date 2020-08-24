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
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// A VM listed on the plan.
type PlanVM struct {
	// The VM identifier.
	// For:
	//   - vSphere: The managed object ID.
	ID string `json:"id"`
	// Enable hooks.
	Hook struct {
		// Run hook before migration.
		Before bool `json:"before,omitempty"`
		// Run hook after migration.
		After bool `json:"after,omitempty"`
	} `json:"hook,omitempty"`
	// Host
	Host core.ObjectReference `json:"host,omitempty" ref:"Host"`
}

//
// PlanSpec defines the desired state of Plan.
type PlanSpec struct {
	// Providers.
	Provider struct {
		// Source.
		Source core.ObjectReference `json:"source" ref:"Provider"`
		// Destination.
		Destination core.ObjectReference `json:"destination" ref:"Provider"`
	} `json:"provider"`
	// Resource map.
	Map core.ObjectReference `json:"map" ref:"Map"`
	// List of VMs.
	VMs []PlanVM `json:"vms"`
}

//
// PlanStatus defines the observed state of Plan.
type PlanStatus struct {
	// Conditions.
	libcnd.Conditions
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Plan struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            PlanSpec   `json:"spec,omitempty"`
	Status          PlanStatus `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlanList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Plan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Plan{}, &PlanList{})
}
