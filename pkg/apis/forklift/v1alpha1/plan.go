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
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/provider"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
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
	Provider provider.Pair `json:"provider"`
	// Resource mapping.
	Map plan.Map `json:"map"`
	// List of VMs.
	VMs []plan.VM `json:"vms"`
	// Whether this is a warm migration.
	Warm bool `json:"warm,omitempty"`
	// Date and time to finalize a warm migration.
	Cutover *meta.Time `json:"cutover,omitempty"`
}

//
// Find a planned VM.
func (r *PlanSpec) FindVM(ref ref.Ref) (v *plan.VM, found bool) {
	for _, vm := range r.VMs {
		if vm.ID == ref.ID {
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
	libcnd.Conditions `json:",inline"`
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
// Get the target namespace.
// Default to `plan` namespace when not specified
// in the plan spec.
func (r *Plan) TargetNamespace() (ns string) {
	ns = r.Spec.TargetNamespace
	if ns == "" {
		ns = r.Plan.Namespace
	}

	return
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
