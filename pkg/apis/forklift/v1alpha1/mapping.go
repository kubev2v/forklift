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
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/provider"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Mapped network destination.
type DestinationNetwork struct {
	// The network type.
	// +kubebuilder:validation:Enum=pod;multus
	Type string `json:"type"`
	// The namespace (multus only).
	Namespace string `json:"namespace,omitempty"`
	// The name.
	Name string `json:"name,omitempty"`
}

//
// Mapped network.
type NetworkPair struct {
	// Source network.
	Source ref.Ref `json:"source"`
	// Destination network.
	Destination DestinationNetwork `json:"destination"`
}

//
// Mapped storage.
type StoragePair struct {
	// Source storage.
	Source ref.Ref `json:"source"`
	// Destination storage.
	Destination DestinationStorage `json:"destination"`
}

//
// Mapped storage destination.
type DestinationStorage struct {
	// A storage class.
	StorageClass string `json:"storageClass"`
	// Volume mode.
	// +kubebuilder:validation:Enum=Filesystem;Block
	VolumeMode core.PersistentVolumeMode `json:"volumeMode,omitempty"`
	// Access mode.
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany;ReadOnlyMany
	AccessMode core.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
}

//
// Network map spec.
type NetworkMapSpec struct {
	// Provider
	Provider provider.Pair `json:"provider"`
	// Map.
	Map []NetworkPair `json:"map"`
}

//
// Storage map spec.
type StorageMapSpec struct {
	// Provider
	Provider provider.Pair `json:"provider"`
	// Map.
	Map []StoragePair `json:"map"`
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
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
}

//
// Find network map for source ID.
func (r *NetworkMap) FindNetwork(networkID string) (pair NetworkPair, found bool) {
	for _, pair = range r.Spec.Map {
		if pair.Source.ID == networkID {
			found = true
			break
		}
	}

	return
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
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
}

//
// Find storage map for source ID.
func (r *StorageMap) FindStorage(storageID string) (pair StoragePair, found bool) {
	for _, pair = range r.Spec.Map {
		if pair.Source.ID == storageID {
			found = true
			break
		}
	}

	return
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
