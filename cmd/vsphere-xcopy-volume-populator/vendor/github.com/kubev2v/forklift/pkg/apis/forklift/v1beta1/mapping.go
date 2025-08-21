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
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mapped network destination.
type DestinationNetwork struct {
	// Type of network to use for the destination.
	// Valid values:
	// - pod: Use the Kubernetes pod network
	// - multus: Use a Multus additional network
	// - ignored: Network is excluded from mapping
	// +kubebuilder:validation:Enum=pod;multus;ignored
	Type string `json:"type"`
	// The namespace (multus only).
	Namespace string `json:"namespace,omitempty"`
	// The name.
	Name string `json:"name,omitempty"`
}

// Mapped network.
type NetworkPair struct {
	// Source network.
	Source ref.Ref `json:"source"`
	// Destination network.
	Destination DestinationNetwork `json:"destination"`
}

// OffloadPlugin is a storage plugin that acts on the storage allocation and copying
// phase of the migration. There can be more than one available but currently only
// one will be supported
type OffloadPlugin struct {
	VSphereXcopyPluginConfig *VSphereXcopyPluginConfig `json:"vsphereXcopyConfig"`
}

// StorageVendorProduct is an identifier of the product used for XCOPY.
// NOTE - Update the kubebuilder:validation line for every change to this enum
type StorageVendorProduct string

const (
	StorageVendorProductVantara        StorageVendorProduct = "vantara"
	StorageVendorProductOntap          StorageVendorProduct = "ontap"
	StorageVendorProductPrimera3Par    StorageVendorProduct = "primera3par"
	StorageVendorProductPureFlashArray StorageVendorProduct = "pureFlashArray"
	StorageVendorProductPowerFlex      StorageVendorProduct = "powerflex"
	StorageVendorProductPowerMax       StorageVendorProduct = "powermax"
	StorageVendorProductPowerStore     StorageVendorProduct = "powerstore"
)

func StorageVendorProducts() []StorageVendorProduct {
	return []StorageVendorProduct{
		StorageVendorProductVantara,
		StorageVendorProductOntap,
		StorageVendorProductPrimera3Par,
		StorageVendorProductPureFlashArray,
		StorageVendorProductPowerFlex,
		StorageVendorProductPowerStore,
		StorageVendorProductPowerMax,
	}
}

// VSphereXcopyPluginConfig works with the Vsphere Xcopy Volume Populator
// to offload the copy to Vsphere and the storage array.
type VSphereXcopyPluginConfig struct {
	// SecretRef is the name of the secret with the storage credentials for the plugin.
	// The secret should reside in the same namespace where the source provider is.
	SecretRef string `json:"secretRef"`
	// StorageVendorProduct the string identifier of the storage vendor product
	// +kubebuilder:validation:Enum=vantara;ontap;primera3par;pureFlashArray;powerflex;powerstore;powermax
	StorageVendorProduct StorageVendorProduct `json:"storageVendorProduct"`
}

// Mapped storage.
type StoragePair struct {
	// Source storage.
	Source ref.Ref `json:"source"`
	// Destination storage.
	Destination DestinationStorage `json:"destination"`
	// Offload Plugin
	OffloadPlugin *OffloadPlugin `json:"offloadPlugin,omitempty"`
}

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

// Network map spec.
type NetworkMapSpec struct {
	// Provider
	Provider provider.Pair `json:"provider"`
	// Map.
	Map []NetworkPair `json:"map"`
}

// Storage map spec.
type StorageMapSpec struct {
	// Provider
	Provider provider.Pair `json:"provider"`
	// Map.
	Map []StoragePair `json:"map"`
}

// MapStatus defines the observed state of Maps.
type MapStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// References.
	ref.Refs `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type NetworkMap struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            NetworkMapSpec `json:"spec,omitempty"`
	Status          MapStatus      `json:"status,omitempty"`
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
}

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

// Find network map for source type.
func (r *NetworkMap) FindNetworkByType(networkType string) (pair NetworkPair, found bool) {
	for _, pair = range r.Spec.Map {
		if pair.Source.Type == networkType {
			found = true
			break
		}
	}

	return
}

// Find network map for source name and namespace.
func (r *NetworkMap) FindNetworkByNameAndNamespace(namespace, name string) (pair NetworkPair, found bool) {
	for _, pair = range r.Spec.Map {
		if pair.Source.Namespace != "" {
			if pair.Source.Namespace == namespace && pair.Source.Name == name {
				found = true
				break
			}
		} else if pair.Source.Name == fmt.Sprintf("%s/%s", namespace, name) {
			found = true
			break
		}
	}

	return
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkMapList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []NetworkMap `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type StorageMap struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            StorageMapSpec `json:"spec,omitempty"`
	Status          MapStatus      `json:"status,omitempty"`
	// Referenced resources populated
	// during validation.
	Referenced `json:"-"`
}

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

// Find storage map for source Name.
func (r *StorageMap) FindStorageByName(storageName string) (pair StoragePair, found bool) {
	for _, pair = range r.Spec.Map {
		if pair.Source.Name == storageName {
			found = true
			break
		}
	}

	return
}

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
