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
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Mapped source.
type MapSourceObject struct {
	// The object identifier.
	// For:
	//   - vsphere: The managed object ID.
	ID string `json:"id"`
}

//
// Mapped network destination.
type MapDestinationNetwork struct {
	// The network type (pod|multus)
	Type string `json:"type"`
	// The namespace (multus only).
	Namespace string `json:"namespace"`
	// The name.
	Name string `json:"name"`
}

//
// Mapped network.
type NetworkPair struct {
	// Source network.
	Source MapSourceObject `json:"source"`
	// Destination network.
	Destination MapDestinationNetwork `json:"destination"`
}

//
// Mapped storage destination.
type MapDestinationStorage struct {
	// A storage class.
	StorageClass string `json:"storageClass"`
}

//
// Mapped storage.
type StoragePair struct {
	// Source storage.
	Source MapSourceObject `json:"source"`
	// Destination storage.
	Destination MapDestinationStorage `json:"destination"`
}

//
// Network map spec.
type NetworkMapSpec struct {
	// Provider
	Provider ProviderPair `json:"provider" ref:"Provider"`
	// Map.
	Map []NetworkPair `json:"map"`
}

//
// Storage map spec.
type StorageMapSpec struct {
	// Provider
	Provider ProviderPair `json:"provider" ref:"Provider"`
	// Map.
	Map []StoragePair `json:"map"`
}

//
// MapStatus defines the observed state of Maps.
type MapStatus struct {
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
