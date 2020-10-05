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
	libitr "github.com/konveyor/controller/pkg/itinerary"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

//
// Plan hook.
type PlanHook struct {
	// Pre-migration hook.
	Before *core.ObjectReference `json:"before,omitempty"`
	// Post-migration hook.
	After *core.ObjectReference `json:"after,omitempty"`
}

//
// A VM listed on the plan.
type PlanVM struct {
	// The VM identifier.
	// For:
	//   - vSphere: The managed object ID.
	ID string `json:"id"`
	// Enable hooks.
	Hook *PlanHook `json:"hook,omitempty"`
	// Host
	Host *core.ObjectReference `json:"host,omitempty" ref:"Host"`
}

//
// Resource map.
type PlanMap struct {
	// Networks.
	Networks []NetworkPair `json:"networks,omitempty"`
	// Datastores.
	Datastores []StoragePair `json:"datastores,omitempty"`
}

//
// Find network map for source ID.
func (r *PlanMap) FindNetwork(networkID string) (pair NetworkPair, found bool) {
	for _, pair = range r.Networks {
		if pair.Source.ID == networkID {
			found = true
			break
		}
	}

	return
}

//
// Find storage map for source ID.
func (r *PlanMap) FindStorage(storageID string) (pair StoragePair, found bool) {
	for _, pair = range r.Datastores {
		if pair.Source.ID == storageID {
			found = true
			break
		}
	}

	return
}

//
// PlanSpec defines the desired state of Plan.
type PlanSpec struct {
	// Description
	Description string `json:"description,omitempty"`
	// Providers.
	Provider ProviderPair `json:"provider"`
	// Resource map.
	Map PlanMap `json:"map,omitempty"`
	// List of VMs.
	VMs []PlanVM `json:"vms"`
}

//
// Find a planned VM.
func (r *PlanSpec) FindVM(vmID string) (v *PlanVM, found bool) {
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
// Pipeline step.
type Step struct {
	// Name.
	Name string `json:"name"`
	// Progress.
	Progress libitr.Progress `json:"progress"`
}

//
// VM errors.
type VMError struct {
	Phase   string   `json:"phase"`
	Reasons []string `json:"reasons"`
}

//
// Migration status.
type PlanMigrationStatus struct {
	// Started timestamp.
	Started *meta.Time `json:"started,omitempty"`
	// Completed timestamp.
	Completed *meta.Time `json:"completed,omitempty"`
	// Provider pair.
	Provider struct {
		// Source.
		Source struct {
			Namespace string       `json:"namespace,omitempty"`
			Name      string       `json:"name,omitempty"`
			Spec      ProviderSpec `json:"spec,omitempty"`
		}
		// Destination.
		Destination struct {
			Namespace string       `json:"namespace,omitempty"`
			Name      string       `json:"name,omitempty"`
			Spec      ProviderSpec `json:"spec,omitempty"`
		}
	} `json:"provider"`
	// Resource map.
	Map PlanMap `json:"map"`
	// Active migration.
	Active types.UID `json:"active"`
	// VM status
	VMs []VMStatus `json:"vms,omitempty"`
}

//
// Set the source provider.
func (r *PlanMigrationStatus) SetSource(provider *Provider) {
	if provider == nil {
		return
	}
	s := &r.Provider.Source
	s.Spec = provider.Spec
	s.Namespace = provider.Namespace
	s.Name = provider.Name
}

//
// Set the destination provider.
func (r *PlanMigrationStatus) SetDestination(provider *Provider) {
	if provider == nil {
		return
	}
	d := &r.Provider.Destination
	d.Spec = provider.Spec
	d.Namespace = provider.Namespace
	d.Name = provider.Name
}

//
// Get the source provider.
func (r *PlanMigrationStatus) GetSource() (provider *Provider) {
	s := &r.Provider.Source
	provider = &Provider{}
	provider.ObjectMeta.Namespace = s.Namespace
	provider.ObjectMeta.Name = s.Name
	provider.Spec = s.Spec
	return
}

//
// Get the destination provider.
func (r *PlanMigrationStatus) GetDestination() (provider *Provider) {
	d := &r.Provider.Destination
	provider = &Provider{}
	provider.ObjectMeta.Namespace = d.Namespace
	provider.ObjectMeta.Name = d.Name
	provider.Spec = d.Spec
	return
}

//
// Find a VM status.
func (r *PlanMigrationStatus) FindVM(vmID string) (v *VMStatus, found bool) {
	for _, vm := range r.VMs {
		if vm.Planned.ID == vmID {
			found = true
			v = &vm
			return
		}
	}

	return
}

//
// VM Status
type VMStatus struct {
	// Planned VM.
	Planned PlanVM `json:"planned"`
	// Migration pipeline.
	Pipeline []Step `json:"pipeline"`
	// Phase
	Phase string `json:"phase"`
	// Started timestamp.
	Started *meta.Time `json:"started,omitempty"`
	// Completed timestamp.
	Completed *meta.Time `json:"completed,omitempty"`
	// Errors
	Error *VMError `json:"error,omitempty"`
}

//
// Find step by name.
func (r *VMStatus) Step(name string) (step *Step, found bool) {
	for i := range r.Pipeline {
		step = &r.Pipeline[i]
		if step.Name == name {
			found = true
			break
		}
	}

	return
}

//
// Pending migration.
func (r *VMStatus) Pending() bool {
	return r.Started == nil
}

//
// Is migrating.
func (r *VMStatus) Migrating() bool {
	return r.Started != nil && r.Completed == nil
}

//
// Migration done.
func (r *VMStatus) Done() bool {
	return r.Completed != nil
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
	Migration PlanMigrationStatus `json:"migration,omitempty"`
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
}

//
// Generated name for kubevirt VM Import mapping CR.
func (r *Plan) NameForMapping() string {
	uid := string(r.GetUID())
	parts := []string{
		"plan",
		r.Name,
		uid[len(uid)-4:],
	}

	return strings.Join(parts, "-")
}

//
// Generated name for kubevirt VM Import CR secret.
func (r *Plan) NameForSecret() string {
	uid := string(r.GetUID())
	parts := []string{
		"plan",
		r.Name,
		uid[len(uid)-4:],
	}

	return strings.Join(parts, "-")
}

//
// Generated name for kubevirt VM Import CR.
func (r *Plan) NameForImport(vmID string) string {
	uid := string(r.Status.Migration.Active)
	parts := []string{
		"plan",
		r.Name,
		vmID,
		uid[len(uid)-4:],
	}

	return strings.Join(parts, "-")
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
