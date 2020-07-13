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
// Provider types.
const (
	// Openshift
	Openshift = "openshift"
	// VMWare
	VMWare = "vmware"
)

//
// Defines the desired state of Provider.
type ProviderSpec struct {
	// Provider type.
	Type string `json:"type"`
	// The (external) provider URL.
	URL string `json:"url"`
	// References a secret containing credentials and
	// other confidential information.
	Secret core.ObjectReference `json:"secret" ref:"Secret"`
}

//
// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
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
type Provider struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ProviderSpec   `json:"spec,omitempty"`
	Status          ProviderStatus `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProviderList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}
