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
// Volume mode.
type VolumeMode struct {
	// Name.
	Name string `json:"name"`
	// Priority
	Priority int `json:"priority"`
	// Feature list.
	Features []string `json:"features,omitempty"`
	// Access modes.
	AccessModes []AccessMode `json:"accessModes,omitempty"`
}

//
// Access mode.
type AccessMode struct {
	// Name.
	Name string `json:"name"`
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProvisionerList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Provisioner `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provisioner{}, &ProvisionerList{})
}
