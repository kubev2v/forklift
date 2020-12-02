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
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// PlanSpec defines the desired state of Plan.
type PlanSpec struct {
	// Description
	Description string `json:"description,omitempty"`
	// Target namespace.
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// Providers.
	Provider ProviderPair `json:"provider"`
	// Resource map.
	Map plan.Map `json:"map,omitempty"`
	// List of VMs.
	VMs []plan.VM `json:"vms"`
}

//
// Find a planned VM.
func (r *PlanSpec) FindVM(vmID string) (v *plan.VM, found bool) {
	for _, vm := range r.VMs {
		if vm.ID == vmID {
			found = true
			v = &vm
			return
		}
	}

	return
}

//
// PlanStatus defines the observed state of Plan.
type PlanStatus struct {
	// Conditions.
	libcnd.Conditions
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Migration
	Migration plan.MigrationStatus `json:"migration,omitempty"`
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
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
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
