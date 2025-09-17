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
	"context"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigrationType defines the type of migration to perform
type MigrationType string

const (
	// Migration types
	MigrationCold           MigrationType = "cold"
	MigrationWarm           MigrationType = "warm"
	MigrationLive           MigrationType = "live"
	MigrationOnlyConversion MigrationType = "conversion"
)

const (
	// namespaceLabelPrimaryUDN is the label key used to identify namespaces with primary user-defined networks
	namespaceLabelPrimaryUDN = "k8s.ovn.org/primary-user-defined-network"
	// nadLabelUDN is the label key used to identify NetworkAttachmentDefinitions that are user-defined networks
	nadLabelUDN = "k8s.ovn.org/user-defined-network"
)

// PlanSpec defines the desired state of Plan.
type PlanSpec struct {
	// Description
	Description string `json:"description,omitempty"`
	// Target namespace.
	TargetNamespace string `json:"targetNamespace"`
	// TargetLabels are labels that should be applied to the target virtual machines.
	// Note: System-managed labels (migration, plan, vmID, app) will override any user-defined
	// labels with the same keys to ensure proper system functionality.
	// See Pod Labels documentation for more details,
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#labels
	TargetLabels map[string]string `json:"targetLabels,omitempty"`
	// TargetNodeSelector, constrains the scheduler to only schedule VMs on nodes,
	// which contain the specified labels.
	// See virtual machine instance NodeSelector documentation for more details,
	// https://kubevirt.io/user-guide/compute/node_assignment/#nodeselector
	TargetNodeSelector map[string]string `json:"targetNodeSelector,omitempty"`
	// TargetAffinity allows specifying hard- and soft-affinity for VMs.
	// it is possible to write matching rules against workloads (VMs and Pods) and Nodes.
	// Since VMs are a workload type based on Pods, Pod-affinity affects VMs as well.
	// See virtual machine instance Affinity documentation for more details,
	// https://kubevirt.io/user-guide/compute/node_assignment/#affinity-and-anti-affinity
	// +structType=atomic
	TargetAffinity *core.Affinity `json:"targetAffinity,omitempty"`
	// ConvertorLabels are labels that should be applied to the virt-v2v convertor pods.
	// The convertor pods run virt-v2v to convert VMware disks, install KVM guest agents and drivers,
	// and handle disk data migration for cold migrations from VMware to KubeVirt.
	// Note: System-managed labels (migration, plan, vmID, forklift.app) will override any user-defined
	// labels with the same keys to ensure proper system functionality.
	// See Pod Labels documentation for more details,
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#labels
	ConvertorLabels map[string]string `json:"convertorLabels,omitempty"`
	// ConvertorNodeSelector constrains the scheduler to only schedule virt-v2v convertor pods on nodes
	// which contain the specified labels. This is useful for dedicating specific nodes for disk conversion
	// workloads that require high I/O performance or network access to source VMware infrastructure.
	// See Pod NodeSelector documentation for more details,
	// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
	ConvertorNodeSelector map[string]string `json:"convertorNodeSelector,omitempty"`
	// ConvertorAffinity allows specifying hard- and soft-affinity for virt-v2v convertor pods.
	// This can be used to optimize placement for disk conversion performance, such as co-locating
	// with storage or ensuring network proximity to VMware infrastructure for cold migration data transfers.
	// See Pod Affinity documentation for more details,
	// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
	// +structType=atomic
	ConvertorAffinity *core.Affinity `json:"convertorAffinity,omitempty"`
	// Providers.
	Provider provider.Pair `json:"provider"`
	// Resource mapping.
	Map plan.Map `json:"map"`
	// List of VMs.
	VMs []plan.VM `json:"vms"`
	// Whether this is a warm migration.
	// Deprecated: this field will be deprecated in 2.10. Use Type instead.
	Warm bool `json:"warm,omitempty"`
	// The network attachment definition that should be used for disk transfer.
	TransferNetwork *core.ObjectReference `json:"transferNetwork,omitempty"`
	// Whether this plan should be archived.
	Archived bool `json:"archived,omitempty"`
	// Preserve the CPU model and flags the VM runs with in its oVirt cluster.
	PreserveClusterCPUModel bool `json:"preserveClusterCpuModel,omitempty"`
	// Preserve static IPs of VMs in vSphere
	// +kubebuilder:default:=true
	PreserveStaticIPs bool `json:"preserveStaticIPs,omitempty"`
	// Deprecated: this field will be deprecated in 2.8.
	DiskBus cnv.DiskBus `json:"diskBus,omitempty"`
	// PVCNameTemplate is a template for generating PVC names for VM disks.
	// It follows Go template syntax and has access to the following variables:
	//   - .VmName: name of the VM
	//   - .PlanName: name of the migration plan
	//   - .DiskIndex: initial volume index of the disk
	//   - .WinDriveLetter: Windows drive letter (lowercase, if applicable, e.g. "c", requires guest agent)
	//   - .RootDiskIndex: index of the root disk
	//   - .Shared: true if the volume is shared by multiple VMs, false otherwise
	//   - .FileName: name of the file in the source provider (VMware only, filename includes the .vmdk suffix)
	// Note:
	//   This template can be overridden at the individual VM level.
	// Examples:
	//   "{{.VmName}}-disk-{{.DiskIndex}}"
	//   "{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data{{end}}-{{.DiskIndex}}"
	//   "{{if .Shared}}shared-{{end}}{{.VmName}}-{{.DiskIndex}}"
	// +optional
	PVCNameTemplate string `json:"pvcNameTemplate,omitempty"`
	// PVCNameTemplateUseGenerateName indicates if the PVC name template should use generateName instead of name.
	// Setting this to false will use the name field of the PVCNameTemplate.
	// This is useful when using a template that generates a name without a suffix.
	// For example, if the template is "{{.VmName}}-disk-{{.DiskIndex}}", setting this to false will result in
	// the PVC name being "{{.VmName}}-disk-{{.DiskIndex}}", which may not be unique.
	// but will be more predictable.
	// **DANGER** When set to false, the generated PVC name may not be unique and may cause conflicts.
	// +optional
	// +kubebuilder:default:=true
	PVCNameTemplateUseGenerateName bool `json:"pvcNameTemplateUseGenerateName,omitempty"`
	// VolumeNameTemplate is a template for generating volume interface names in the target virtual machine.
	// It follows Go template syntax and has access to the following variables:
	//   - .PVCName: name of the PVC mounted to the VM using this volume
	//   - .VolumeIndex: sequential index of the volume interface (0-based)
	// Note:
	//   - This template can be overridden at the individual VM level
	//   - If not specified on VM level and on Plan leverl, default naming conventions will be used
	// Examples:
	//   "disk-{{.VolumeIndex}}"
	//   "pvc-{{.PVCName}}"
	// +optional
	VolumeNameTemplate string `json:"volumeNameTemplate,omitempty"`
	// NetworkNameTemplate is a template for generating network interface names in the target virtual machine.
	// It follows Go template syntax and has access to the following variables:
	//   - .NetworkName: If target network is multus, name of the Multus network attachment definition, empty otherwise.
	//   - .NetworkNamespace: If target network is multus, namespace where the network attachment definition is located.
	//   - .NetworkType: type of the network ("Multus" or "Pod")
	//   - .NetworkIndex: sequential index of the network interface (0-based)
	// The template can be used to customize network interface names based on target network configuration.
	// Note:
	//   - This template can be overridden at the individual VM level
	//   - If not specified on VM level and on Plan leverl, default naming conventions will be used
	// Examples:
	//   "net-{{.NetworkIndex}}"
	//   "{{if eq .NetworkType "Pod"}}pod{{else}}multus-{{.NetworkIndex}}{{end}}"
	// +optional
	NetworkNameTemplate string `json:"networkNameTemplate,omitempty"`
	// Determines if the plan should migrate shared disks.
	// +kubebuilder:default:=true
	MigrateSharedDisks bool `json:"migrateSharedDisks,omitempty"`
	// DeleteGuestConversionPod determines if the guest conversion pod should be deleted after successful migration.
	// Note:
	//   - If this option is enabled and migration succeeds then the pod will get deleted. However the VM could still not boot and the virt-v2v logs, with additional information, will be deleted alongside guest conversion pod.
	//   - If migration fails the conversion pod will remain present even if this option is enabled.
	// +optional
	DeleteGuestConversionPod bool `json:"deleteGuestConversionPod,omitempty"`
	// DeleteVmOnFailMigration controls whether the target VM created by this Plan is deleted when a migration fails.
	// When true and the migration fails after the target VM has been created, the controller
	// will delete the target VM (and related target-side resources) during failed-migration cleanup
	// and when the Plan is deleted. When false (default), the target VM is preserved to aid
	// troubleshooting. The source VM is never modified.
	//
	// Note: If the Plan-level option is set to true, the VM-level option will be ignored.
	//
	// +optional
	DeleteVmOnFailMigration bool `json:"deleteVmOnFailMigration,omitempty"`
	// InstallLegacyDrivers determines whether to install legacy windows drivers in the VM.
	//The following Vm's are lack of SHA-2 support and need legacy drivers:
	// Windows XP (all)
	// Windows Server 2003
	// Windows Vista (all)
	// Windows Server 2008
	// Windows 7 (pre-SP1)
	// Windows Server 2008 R2
	// Behavior:
	// - If set to nil (unset), the system will automatically detect whether the VM requires legacy drivers
	//   based on its guest OS type (using IsLegacyWindows).
	// - If set to true, legacy drivers will be installed unconditionally by setting the VIRTIO_WIN environment variable.
	// - If set to false, legacy drivers will be skipped, and the system will fall back to using the standard (SHA-2 signed) drivers.
	//
	// When enabled, legacy drivers are exposed to the virt-v2v conversion process via the VIRTIO_WIN environment variable,
	// which points to the legacy ISO at /usr/local/virtio-win-legacy.iso.
	InstallLegacyDrivers *bool `json:"installLegacyDrivers,omitempty"`
	// Determines if the plan should skip the guest conversion.
	// +kubebuilder:default:=false
	SkipGuestConversion bool `json:"skipGuestConversion,omitempty"`
	// useCompatibilityMode controls whether to use VirtIO devices when skipGuestConversion is true (Raw Copy mode).
	// This setting has no effect when skipGuestConversion is false (V2V Conversion always uses VirtIO).
	// - true (default): Use compatibility devices (SATA bus, E1000E NIC) to ensure bootability
	// - false: Use high-performance VirtIO devices (requires VirtIO drivers already installed in source VM)
	// +kubebuilder:default:=true
	UseCompatibilityMode bool `json:"useCompatibilityMode,omitempty"`
	// Migration type. e.g. "cold", "warm", "live", "conversion". Supersedes the `warm` boolean if set.
	// +optional
	// +kubebuilder:validation:Enum=cold;warm;live;conversion
	Type MigrationType `json:"type,omitempty"`
	// TargetPowerState specifies the desired power state of the target VM after migration.
	// - "on": Target VM will be powered on after migration
	// - "off": Target VM will be powered off after migration
	// - "auto" or nil (default): Target VM will match the source VM's power state
	// +optional
	// +kubebuilder:validation:Enum=on;off;auto
	TargetPowerState plan.TargetPowerState `json:"targetPowerState,omitempty"`
	// RunPreflightInspection controls if an inspection step on VM base disks should be performed before starting the first disk transfer. Only applies to the warm migration type.
	// - true (default): Inspection step will be run before transfering any disks. This step can fail if inspection finds out that migration can fail.
	// - false: No inspection will be run before disk transfer.
	// +kubebuilder:default:=true
	RunPreflightInspection bool `json:"runPreflightInspection,omitempty"`
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

// If the plan calls for the vm to be cold migrated to the local cluster, we can
// just use virt-v2v directly to convert the vm while copying data over. In other
// cases, we use CDI to transfer disks to the destination cluster and then use
// virt-v2v-in-place to convert these disks after cutover.
func (p *Plan) ShouldUseV2vForTransfer() (bool, error) {
	source := p.Referenced.Provider.Source
	if source == nil {
		return false, liberr.New("Cannot analyze plan, source provider is missing.")
	}
	destination := p.Referenced.Provider.Destination
	if destination == nil {
		return false, liberr.New("Cannot analyze plan, destination provider is missing.")
	}

	switch source.Type() {
	case VSphere:
		// The virt-v2v transferes all disks attached to the VM. If we want to skip the shared disks so we don't transfer
		// them multiple times we need to manage the transfer using KubeVirt CDI DataVolumes and v2v-in-place.
		return !p.Spec.Warm && // The Warm Migraiton needs to use CDI to manage the snapshot delta
				destination.IsHost() && // We can't monitor progress from the guest converison pod on the remote clusters
				p.Spec.MigrateSharedDisks && // virt-v2v migrates all disks, to skip shared we need to control the disk selection
				!p.Spec.SkipGuestConversion && // virt-v2v always converts the guest, to perform RawCopyMode we need to copy just disks via CDI
				p.Spec.Type != MigrationOnlyConversion, // For only v2v-in-place conversion, we don't want to populate disks by v2v
			nil
	case Ova:
		return true, nil
	default:
		return false, nil
	}
}

func (r *Plan) DestinationHasUdnNetwork(client k8sclient.Client) bool {
	key := k8sclient.ObjectKey{
		Name: r.Spec.TargetNamespace,
	}
	namespace := &core.Namespace{}
	err := client.Get(context.TODO(), key, namespace)
	if err != nil {
		return false
	}
	_, hasUdnLabel := namespace.ObjectMeta.Labels[namespaceLabelPrimaryUDN]
	if !hasUdnLabel {
		return false
	}

	nadList := &k8snet.NetworkAttachmentDefinitionList{}
	listOpts := []k8sclient.ListOption{
		k8sclient.InNamespace(r.Spec.TargetNamespace),
		k8sclient.MatchingLabels{nadLabelUDN: ""},
	}

	err = client.List(context.TODO(), nadList, listOpts...)
	if err != nil {
		return false
	}

	if len(nadList.Items) > 0 {
		return true
	}
	return false
}

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

// PVCNameTemplateData contains fields used in naming templates.
type PVCNameTemplateData struct {
	VmName         string `json:"vmName"`
	PlanName       string `json:"planName"`
	DiskIndex      int    `json:"diskIndex"`
	WinDriveLetter string `json:"winDriveLetter,omitempty"`
	RootDiskIndex  int    `json:"rootDiskIndex"`
	Shared         bool   `json:"shared,omitempty"`
	FileName       string `json:"fileName,omitempty"`
}

// VolumeNameTemplateData contains fields used in naming templates.
type VolumeNameTemplateData struct {
	PVCName     string `json:"pvcName,omitempty"`
	VolumeIndex int    `json:"volumeIndex,omitempty"`
}

// NetworkNameTemplateData contains fields used in naming templates.
type NetworkNameTemplateData struct {
	// NetworkName is the name of the Multus network attachment definition if target network is multus, empty otherwise
	NetworkName string `json:"networkName,omitempty"`
	// NetworkNamespace is the namespace where the network attachment definition is located if target network is multus
	NetworkNamespace string `json:"networkNamespace,omitempty"`
	// NetworkType is the type of the network ("Multus" or "Pod")
	NetworkType string `json:"networkType,omitempty"`
	// NetworkIndex is the sequential index of the network interface (0-based)
	NetworkIndex int `json:"networkIndex,omitempty"`
}
