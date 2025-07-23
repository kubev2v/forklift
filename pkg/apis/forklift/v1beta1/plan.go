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

package v1beta1

import (
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/provider"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PlanSpec defines the desired state of Plan.
type PlanSpec struct {
	// Description
	Description string `json:"description,omitempty"`
	// Target namespace.
	TargetNamespace string `json:"targetNamespace"`
	// Providers.
	Provider provider.Pair `json:"provider"`
	// Resource mapping.
	Map plan.Map `json:"map"`
	// List of VMs.
	VMs []plan.VM `json:"vms"`
	// Whether this is a warm migration.
	Warm bool `json:"warm,omitempty"`
	// The network attachment definition that should be used for disk transfer.
	TransferNetwork *core.ObjectReference `json:"transferNetwork,omitempty"`
	// Whether this plan should be archived.
	Archived bool `json:"archived,omitempty"`
	// Preserve the CPU model and flags the VM runs with in its oVirt cluster.
	PreserveClusterCPUModel bool `json:"preserveClusterCpuModel,omitempty"`
	// Preserve static IPs of VMs in vSphere
	PreserveStaticIPs bool `json:"preserveStaticIPs,omitempty"`
}

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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXECUTING",type=string,JSONPath=".status.conditions[?(@.type=='Executing')].status"
// +kubebuilder:printcolumn:name="SUCCEEDED",type=string,JSONPath=".status.conditions[?(@.type=='Succeeded')].status"
// +kubebuilder:printcolumn:name="FAILED",type=string,JSONPath=".status.conditions[?(@.type=='Failed')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Plan struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            PlanSpec   `json:"spec,omitempty"`
	Status          PlanStatus `json:"status,omitempty"`
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
}

// // If the plan calls for the vm to be cold migrated to the local cluster, we can
// // just use virt-v2v directly to convert the vm while copying data over. In other
// // cases, we use CDI to transfer disks to the destination cluster and then use
// // virt-v2v-in-place to convert these disks after cutover.
// func (p *Plan) VSphereColdLocal() (bool, error) {
// 	source := p.Referenced.Provider.Source
// 	if source == nil {
// 		return false, liberr.New("Cannot analyze plan, source provider is missing.")
// 	}
// 	destination := p.Referenced.Provider.Destination
// 	if destination == nil {
// 		return false, liberr.New("Cannot analyze plan, destination provider is missing.")
// 	}

// 	switch source.Type() {
// 	case VSphere:
// 		return !p.Spec.Warm && destination.IsHost(), nil
// 	case Ova:
// 		return true, nil
// 	default:
// 		return false, nil
// 	}
// }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlanList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Plan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Plan{}, &PlanList{})
}

func (r *Plan) IsSourceProviderOpenstack() bool {
	return r.Provider.Source.Type() == OpenStack
}

func (r *Plan) IsSourceProviderOvirt() bool {
	return r.Provider.Source.Type() == OVirt
}

func (r *Plan) IsSourceProviderOCP() bool {
	return r.Provider.Source.Type() == OpenShift
}

func (r *Plan) IsSourceProviderVSphere() bool { return r.Provider.Source.Type() == VSphere }

func (r *Plan) IsSourceProviderOVA() bool {
	return r.Provider.Source.Type() == Ova
}
