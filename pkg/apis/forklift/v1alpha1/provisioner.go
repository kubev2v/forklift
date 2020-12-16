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
	"sort"
)

//
// Volume mode.
type VolumeMode struct {
	// Name.
	// +kubebuilder:validation:Enum=Filesystem;Block
	Name core.PersistentVolumeMode `json:"name"`
	// Priority
	Priority int `json:"priority"`
	// Feature list.
	Features []string `json:"features,omitempty"`
	// Access modes.
	AccessModes []AccessMode `json:"accessModes,omitempty"`
}

//
// Find accessMode by name.
// Returns the `default` when not found.
// The default is the mode with the lowest priority.
func (r *VolumeMode) AccessMode(name core.PersistentVolumeAccessMode) (m *AccessMode) {
	list := r.AccessModes
	sort.Slice(
		list,
		func(i, j int) bool {
			return list[i].Priority < list[j].Priority
		})
	for i := range list {
		m = &list[i]
		if m.Name == name {
			return
		}
	}
	if len(list) == 0 {
		m = &AccessMode{Name: name}
	} else {
		m = &list[0]
	}

	return
}

//
// Access mode.
type AccessMode struct {
	// Name.
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany;ReadOnlyMany
	Name core.PersistentVolumeAccessMode `json:"name"`
	// Priority
	Priority int `json:"priority"`
	// Feature list.
	Features []string `json:"features,omitempty"`
}

//
// ProvisionerSpec defines the desired state of Provisioner
type ProvisionerSpec struct {
	Name        string       `json:"name"`
	Features    []string     `json:"features,omitempty"`
	VolumeModes []VolumeMode `json:"volumeModes,omitempty"`
}

//
// ProvisionerStatus defines the observed state of Provisioner
type ProvisionerStatus struct {
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
type Provisioner struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ProvisionerSpec   `json:"spec,omitempty"`
	Status          ProvisionerStatus `json:"status,omitempty"`
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
}

//
// Find volumeMode by name.
// Returns the `default` when not found.
// The default is the mode with the lowest priority.
func (r *Provisioner) VolumeMode(name core.PersistentVolumeMode) (m *VolumeMode) {
	list := r.Spec.VolumeModes
	sort.Slice(
		list,
		func(i, j int) bool {
			return list[i].Priority < list[j].Priority
		})
	for i := range list {
		m = &list[i]
		if m.Name == name {
			return
		}
	}
	if len(list) == 0 {
		m = &VolumeMode{Name: name}
	} else {
		m = &list[0]
	}

	return
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProvisionerList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Provisioner `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provisioner{}, &ProvisionerList{})
}
