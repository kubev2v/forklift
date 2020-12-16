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
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/mapped"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Network map spec.
type NetworkMapSpec struct {
	// Provider
	Provider ProviderPair `json:"provider" ref:"Provider"`
	// Map.
	Map []mapped.NetworkPair `json:"map"`
}

//
// Storage map spec.
type StorageMapSpec struct {
	// Provider
	Provider ProviderPair `json:"provider" ref:"Provider"`
	// Map.
	Map []mapped.StoragePair `json:"map"`
}

//
// MapStatus defines the observed state of Maps.
type MapStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type NetworkMap struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            NetworkMapSpec `json:"spec,omitempty"`
	Status          MapStatus      `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkMapList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []NetworkMap `json:"items"`
}

//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type StorageMap struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            StorageMapSpec `json:"spec,omitempty"`
	Status          MapStatus      `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StorageMapList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []StorageMap `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&NetworkMap{},
		&NetworkMapList{},
		&StorageMap{},
		&StorageMapList{})
}
