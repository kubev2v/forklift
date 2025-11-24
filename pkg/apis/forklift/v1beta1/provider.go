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
	"os"
	"strconv"

	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProviderType string

// Provider types.
const (
	Undefined ProviderType = ""
	// OpenShift
	OpenShift ProviderType = "openshift"
	// vSphere
	VSphere ProviderType = "vsphere"
	// oVirt
	OVirt ProviderType = "ovirt"
	// OpenStack
	OpenStack ProviderType = "openstack"
	// OVA
	Ova ProviderType = "ova"
)

var ProviderTypes = []ProviderType{
	OpenShift,
	VSphere,
	OVirt,
	OpenStack,
	Ova,
}

func (t ProviderType) String() string {
	return string(t)
}

// Secret fields.
const (
	Insecure = "insecureSkipVerify"
	Token    = "token"
)

// Provider settings.
const (
	VDDK                   = "vddkInitImage"
	SDK                    = "sdkEndpoint"
	VCenter                = "vcenter"
	ESXI                   = "esxi"
	UseVddkAioOptimization = "useVddkAioOptimization"
	VddkConfig             = "vddkConfig"
	ESXiCloneMethod        = "esxiCloneMethod"
)

const DynamicProviderFinalizer = "forklift/dynamic-provider"
const OvaProviderFinalizer = "forklift/ova-provider"

// Defines the desired state of Provider.
type ProviderSpec struct {
	// Provider type.
	Type *ProviderType `json:"type"`
	// The provider URL.
	// Empty may be used for the `host` provider.
	URL string `json:"url,omitempty"`
	// References a secret containing credentials and
	// other confidential information.
	Secret core.ObjectReference `json:"secret" ref:"Secret"`
	// Provider settings.
	Settings map[string]string `json:"settings,omitempty"`
	// Volumes to mount in the provider's plugin server pod.
	// These are NOT created by the controller - they must already exist or be inline sources.
	// Supports any standard Kubernetes VolumeSource (NFS, existing PVC, ConfigMap, Secret, etc.).
	// Use this for mounting existing data sources like NFS shares with VM files or config data.
	// For dynamic storage that the controller should create, use DynamicProvider.spec.storages instead.
	// +optional
	Volumes []ProviderVolume `json:"volumes,omitempty"`
	// Node selector for scheduling the provider server pod.
	// Only applies to dynamic provider servers.
	// +optional
	ServerNodeSelector map[string]string `json:"serverNodeSelector,omitempty"`
	// Affinity rules for scheduling the provider server pod.
	// Only applies to dynamic provider servers.
	// +optional
	ServerAffinity *core.Affinity `json:"serverAffinity,omitempty"`
}

// ProviderVolume defines a volume to mount in the provider's plugin server pod.
// This is a reference to an existing volume source or inline volume definition.
// The controller does NOT create any resources for these volumes - they must already exist
// (for PVCs, ConfigMaps, Secrets) or be inline definitions (NFS, HostPath, etc.).
// Think of this as equivalent to volumes[] in a Pod spec.
type ProviderVolume struct {
	// Name of the volume. Must be unique within the provider.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Path where the volume should be mounted in the container.
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`

	// Subpath within the volume to mount (optional).
	// +optional
	SubPath string `json:"subPath,omitempty"`

	// Mount as read-only if true, read-write otherwise (defaults to false).
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`

	// Standard Kubernetes VolumeSource - can be any type supported by Kubernetes.
	// Common examples: nfs, persistentVolumeClaim, configMap, secret, emptyDir.
	// This is embedded directly in the Pod spec - no separate resources are created.
	// +kubebuilder:validation:Required
	VolumeSource core.VolumeSource `json:"source"`
}

// ProviderFeatures defines feature flags for dynamic providers.
// Dynamic providers declare their capabilities via these flags.
type ProviderFeatures struct {
	// Indicates if this provider requires guest OS conversion during migration.
	// When true, virt-v2v conversion pods are created for disk format conversion.
	// +optional
	RequiresConversion bool `json:"requiresConversion,omitempty"`
	// Supported migration types for this provider.
	// Valid values: "cold", "warm", "live", "conversion"
	// If empty, defaults to ["cold"]
	// +optional
	SupportedMigrationTypes []MigrationType `json:"supportedMigrationTypes,omitempty"`
	// Indicates if this provider implements a custom VM spec builder.
	// When true, the controller calls POST /vms/{id}/build-spec to get the VM spec.
	// When false, the controller uses the generic builder.
	// +optional
	SupportsCustomBuilder bool `json:"supportsCustomBuilder,omitempty"`
}

// ServiceEndpoint contains connection information for a provider service endpoint.
// This is used to tell the proxy where to forward requests for this provider.
type ServiceEndpoint struct {
	// Name of the service.
	Name string `json:"name"`
	// Namespace of the service.
	Namespace string `json:"namespace"`
	// Port of the service (defaults to 8080 if not specified).
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// Current life cycle phase of the provider.
	// +optional
	Phase string `json:"phase,omitempty"`
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Fingerprint.
	// +optional
	Fingerprint string `json:"fingerprint,omitempty"`
	// Service endpoint for this provider.
	// Used by the proxy to forward inventory and migration requests.
	// +optional
	Service *ServiceEndpoint `json:"service,omitempty"`
	// Feature flags for dynamic providers.
	// +optional
	Features *ProviderFeatures `json:"features,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="CONNECTED",type=string,JSONPath=".status.conditions[?(@.type=='ConnectionTestSucceeded')].status"
// +kubebuilder:printcolumn:name="INVENTORY",type=string,JSONPath=".status.conditions[?(@.type=='InventoryCreated')].status"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.url"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Provider struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ProviderSpec   `json:"spec,omitempty"`
	Status          ProviderStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProviderList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}

// The provider type.
func (p *Provider) Type() ProviderType {
	if p.Spec.Type != nil {
		return *p.Spec.Type
	}
	return Undefined
}

func (p *Provider) SupportsPreserveStaticIps() bool {
	return p.Type() == VSphere
}

// This provider is the `host` cluster.
func (p *Provider) IsHost() bool {
	return p.Type() == OpenShift && p.Spec.URL == ""
}

// This provider is a `host` provider but it is not within the main forklift
// namespace (e.g. generally 'konveyor-forklift' or 'openshift-mtv'). All other
// 'host' providers are namespace-scoped and should use limited credentials
func (p *Provider) IsRestrictedHost() bool {
	return p.IsHost() && p.GetNamespace() != os.Getenv("POD_NAMESPACE")
}

// Current generation has been reconciled.
func (p *Provider) HasReconciled() bool {
	return p.Generation == p.Status.ObservedGeneration
}

// This provider requires VM guest conversion.
func (p *Provider) RequiresConversion() bool {
	// Check if this is a static provider type that requires conversion
	if p.Type() == VSphere || p.Type() == Ova {
		return true
	}
	// For dynamic providers, check the feature flags
	if p.Status.Features != nil {
		return p.Status.Features.RequiresConversion
	}
	return false
}

// This provider support the vddk aio parameters.
func (p *Provider) UseVddkAioOptimization() bool {
	useVddkAioOptimization := p.Spec.Settings[UseVddkAioOptimization]
	if useVddkAioOptimization == "" {
		return false
	}
	parseBool, err := strconv.ParseBool(useVddkAioOptimization)
	if err != nil {
		return false
	}
	return parseBool
}
