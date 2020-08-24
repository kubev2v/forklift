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
// Mapped network.
type NetworkPair struct {
	// Source network.
	Source struct {
		// The network identifier.
		// For:
		//   - vsphere: The managed object ID.
		ID string `json:"id"`
	} `json:"source"`
	// Destination network.
	Destination struct {
		// The network type (pod|multus)
		Type string `json:"type"`
		// The namespace (multus only).
		Namespace string `json:"namespace"`
		// The name.
		Name string `json:"name"`
	} `json:"destination"`
}

//
// Mapped storage.
type StoragePair struct {
	// Source storage.
	Source struct {
		// The storage identifier.
		// For:
		//   - vsphere: The managed object ID.
		ID string `json:"id"`
	} `json:"source"`
	// Destination storage.
	Destination struct {
		// A storage class.
		StorageClass string `json:"storageClass"`
	} `json:"destination"`
}

//
// MapSpec defines the desired state of Map.
type MapSpec struct {
	// Provider
	Provider core.ObjectReference `json:"provider" ref:"Provider"`
	// Network map.
	Networks []NetworkPair `json:"networks"`
	// Datastore map.
	Datastores []StoragePair `json:"datastores"`
}

//
// MapStatus defines the observed state of Map.
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
type Map struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            MapSpec   `json:"spec,omitempty"`
	Status          MapStatus `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MapList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Map `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Map{}, &MapList{})
}
