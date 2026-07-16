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
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ForkliftControllerSpec defines the desired state of ForkliftController.
type ForkliftControllerSpec struct {

	// Feature Gates

	// Enable UI plugin.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureUIPlugin string `json:"feature_ui_plugin,omitempty"`
	// Enable validation service.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureValidation string `json:"feature_validation,omitempty"`
	// Enable volume populators.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureVolumePopulator string `json:"feature_volume_populator,omitempty"`
	// Enable copy offload plugins.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureCopyOffload string `json:"feature_copy_offload,omitempty"`
	// Enable OCP live migration.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureOCPLiveMigration string `json:"feature_ocp_live_migration,omitempty"`
	// Enable OVF-based appliance management endpoints (OVA, HyperV).
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureOVAApplianceManagement string `json:"feature_ova_appliance_management,omitempty"`
	// Use VMware system serial numbers.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	FeatureVMwareSystemSerialNumber string `json:"feature_vmware_system_serial_number,omitempty"`
	// Require authentication.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	FeatureAuthRequired string `json:"feature_auth_required,omitempty"`
	// Enable CLI download service.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureCLIDownload string `json:"feature_cli_download,omitempty"`
	// Enable MCP server deployment (requires OpenShift with Lightspeed).
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureMCPServer string `json:"feature_mcp_server,omitempty"`
	// Run VMware driver removal scripts during Windows vSphere conversion.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	FeatureVSphereVMwareDriverRemoval string `json:"feature_vsphere_vmware_driver_removal,omitempty"`
	// Use registry-based network configuration scripts for Windows static IP setup.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureWindowsRegistryNetworkConfig string `json:"feature_windows_registry_network_config,omitempty"`
	// Enable automatic wait-for-reboot step for Windows VM migrations.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureWindowsWaitForReboot string `json:"feature_windows_wait_for_reboot,omitempty"`
	// Delegate VM conversion to Conversion CRs instead of managing it directly.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	FeatureUseConversionCR string `json:"feature_use_conversion_cr,omitempty"`

	// Container Images

	// Controller pod image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerImageFQIN string `json:"controller_image_fqin,omitempty"`
	// Virt-v2v conversion image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VImageFQIN string `json:"virt_v2v_image_fqin,omitempty"`
	// Virt-v2v XFS conversion image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VImageXFSFQIN string `json:"virt_v2v_image_xfs_fqin,omitempty"`
	// Deep inspection image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	DeepInspectionImageFQIN string `json:"deep_inspection_image_fqin,omitempty"`
	// Deep inspection XFS image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	DeepInspectionImageXFSFQIN string `json:"deep_inspection_image_xfs_fqin,omitempty"`
	// VDDK image for VMware disk access. Optional. If left empty, no VDDK image is configured.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VDDKImage string `json:"vddk_image,omitempty"`
	// API service image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	APIImageFQIN string `json:"api_image_fqin,omitempty"`
	// CLI download service image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	CLIDownloadImageFQIN string `json:"cli_download_image_fqin,omitempty"`
	// UI plugin image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	UIPluginImageFQIN string `json:"ui_plugin_image_fqin,omitempty"`
	// Validation service image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationImageFQIN string `json:"validation_image_fqin,omitempty"`
	// Volume populator controller image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorControllerImageFQIN string `json:"populator_controller_image_fqin,omitempty"`
	// oVirt populator image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorOvirtImageFQIN string `json:"populator_ovirt_image_fqin,omitempty"`
	// OpenStack populator image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorOpenstackImageFQIN string `json:"populator_openstack_image_fqin,omitempty"`
	// vSphere xcopy populator image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorVSphereCopyOffloadImageFQIN string `json:"populator_vsphere_copy_offload_image_fqin,omitempty"`
	// OVA provider server image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProviderServerFQIN string `json:"ova_provider_server_fqin,omitempty"`
	// HyperV provider server image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HyperVProviderServerFQIN string `json:"hyperv_provider_server_fqin,omitempty"`
	// Must-gather debugging image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MustGatherImageFQIN string `json:"must_gather_image_fqin,omitempty"`
	// OVA inventory proxy image. Optional. If left empty, the operator automatically sets this from the release payload.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProxyFQIN string `json:"ova_proxy_fqin,omitempty"`

	// Controller Resource Configuration

	// Controller CPU limit.
	// +optional
	// +kubebuilder:default="2"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerContainerLimitsCPU string `json:"controller_container_limits_cpu,omitempty"`
	// Controller memory limit.
	// +optional
	// +kubebuilder:default="800Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerContainerLimitsMemory string `json:"controller_container_limits_memory,omitempty"`
	// Controller CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerContainerRequestsCPU string `json:"controller_container_requests_cpu,omitempty"`
	// Controller memory request.
	// +optional
	// +kubebuilder:default="350Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerContainerRequestsMemory string `json:"controller_container_requests_memory,omitempty"`
	// Optional NAD name for controller pod transfer network (format: 'namespace/network-name').
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerTransferNetwork string `json:"controller_transfer_network,omitempty"`

	// Inventory Resource Configuration

	// Inventory CPU limit.
	// +optional
	// +kubebuilder:default="2"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	InventoryContainerLimitsCPU string `json:"inventory_container_limits_cpu,omitempty"`
	// Inventory memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	InventoryContainerLimitsMemory string `json:"inventory_container_limits_memory,omitempty"`
	// Inventory CPU request.
	// +optional
	// +kubebuilder:default="500m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	InventoryContainerRequestsCPU string `json:"inventory_container_requests_cpu,omitempty"`
	// Inventory memory request.
	// +optional
	// +kubebuilder:default="500Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	InventoryContainerRequestsMemory string `json:"inventory_container_requests_memory,omitempty"`
	// Inventory route timeout.
	// +optional
	// +kubebuilder:default="360s"
	// +kubebuilder:validation:Pattern="^[0-9]+(s|m|h)$"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	InventoryRouteTimeout string `json:"inventory_route_timeout,omitempty"`

	// API Resource Configuration

	// API service CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	APIContainerLimitsCPU string `json:"api_container_limits_cpu,omitempty"`
	// API service memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	APIContainerLimitsMemory string `json:"api_container_limits_memory,omitempty"`
	// API service CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	APIContainerRequestsCPU string `json:"api_container_requests_cpu,omitempty"`
	// API service memory request.
	// +optional
	// +kubebuilder:default="150Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	APIContainerRequestsMemory string `json:"api_container_requests_memory,omitempty"`

	// CLI Download Resource Configuration

	// CLI download service CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	CLIDownloadContainerLimitsCPU string `json:"cli_download_container_limits_cpu,omitempty"`
	// CLI download service memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	CLIDownloadContainerLimitsMemory string `json:"cli_download_container_limits_memory,omitempty"`
	// CLI download service CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	CLIDownloadContainerRequestsCPU string `json:"cli_download_container_requests_cpu,omitempty"`
	// CLI download service memory request.
	// +optional
	// +kubebuilder:default="512Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	CLIDownloadContainerRequestsMemory string `json:"cli_download_container_requests_memory,omitempty"`

	// MCP Server Lightspeed Integration

	// Register MCP server with OpenShift Lightspeed.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	MCPServerLightspeedIntegration string `json:"mcp_server_lightspeed_integration,omitempty"`
	// Add MCPServer to OLSConfig featureGates when registering with Lightspeed.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:radio:true","urn:alm:descriptor:com.tectonic.ui:radio:false"}
	MCPServerLightspeedSetMCPGate string `json:"mcp_server_lightspeed_set_mcp_gate,omitempty"`

	// MCP Server Resource Configuration

	// MCP server CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerContainerLimitsCPU string `json:"mcp_server_container_limits_cpu,omitempty"`
	// MCP server memory limit.
	// +optional
	// +kubebuilder:default="512Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerContainerLimitsMemory string `json:"mcp_server_container_limits_memory,omitempty"`
	// MCP server CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerContainerRequestsCPU string `json:"mcp_server_container_requests_cpu,omitempty"`
	// MCP server memory request.
	// +optional
	// +kubebuilder:default="256Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerContainerRequestsMemory string `json:"mcp_server_container_requests_memory,omitempty"`

	// MCP Server Settings

	// MCP server output format.
	// +optional
	// +kubebuilder:default="markdown"
	// +kubebuilder:validation:Enum=markdown;text;json
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerOutputFormat string `json:"mcp_server_output_format,omitempty"`
	// MCP server max response chars, 0 for unlimited.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerMaxResponseChars *int32 `json:"mcp_server_max_response_chars,omitempty"`
	// Skip TLS verification for in-cluster MCP calls.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerKubeInsecure string `json:"mcp_server_kube_insecure,omitempty"`
	// Run MCP server in read-only mode.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerReadOnly string `json:"mcp_server_read_only,omitempty"`
	// MCP server verbosity level.
	// +optional
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MCPServerVerbose *int32 `json:"mcp_server_verbose,omitempty"`

	// UI Plugin Resource Configuration

	// UI plugin CPU limit.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	UIPluginContainerLimitsCPU string `json:"ui_plugin_container_limits_cpu,omitempty"`
	// UI plugin memory limit.
	// +optional
	// +kubebuilder:default="800Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	UIPluginContainerLimitsMemory string `json:"ui_plugin_container_limits_memory,omitempty"`
	// UI plugin CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	UIPluginContainerRequestsCPU string `json:"ui_plugin_container_requests_cpu,omitempty"`
	// UI plugin memory request.
	// +optional
	// +kubebuilder:default="150Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	UIPluginContainerRequestsMemory string `json:"ui_plugin_container_requests_memory,omitempty"`

	// Validation Resource Configuration

	// Validation CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationContainerLimitsCPU string `json:"validation_container_limits_cpu,omitempty"`
	// Validation memory limit.
	// +optional
	// +kubebuilder:default="300Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationContainerLimitsMemory string `json:"validation_container_limits_memory,omitempty"`
	// Validation CPU request.
	// +optional
	// +kubebuilder:default="400m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationContainerRequestsCPU string `json:"validation_container_requests_cpu,omitempty"`
	// Validation memory request.
	// +optional
	// +kubebuilder:default="50Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationContainerRequestsMemory string `json:"validation_container_requests_memory,omitempty"`
	// Policy agent search interval in seconds.
	// +optional
	// +kubebuilder:default=120
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationPolicyAgentSearchInterval *int32 `json:"validation_policy_agent_search_interval,omitempty"`
	// Volume name for extra validation rules.
	// +optional
	// +kubebuilder:default="validation-extra-rules"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationExtraVolumeName string `json:"validation_extra_volume_name,omitempty"`
	// Mount path for extra validation rules.
	// +optional
	// +kubebuilder:default="/usr/share/opa/policies/extra"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ValidationExtraVolumeMountpath string `json:"validation_extra_volume_mountpath,omitempty"`

	// ConfigMap Names

	// ConfigMap name for oVirt OS mappings.
	// +optional
	// +kubebuilder:default="forklift-ovirt-osmap"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OvirtOSMapConfigMapName string `json:"ovirt_osmap_configmap_name,omitempty"`
	// ConfigMap name for vSphere OS mappings.
	// +optional
	// +kubebuilder:default="forklift-vsphere-osmap"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VSphereOSMapConfigMapName string `json:"vsphere_osmap_configmap_name,omitempty"`
	// ConfigMap name for virt-customize configuration.
	// +optional
	// +kubebuilder:default="forklift-virt-customize"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtCustomizeConfigMapName string `json:"virt_customize_configmap_name,omitempty"`
	// ConfigMap name for custom NAA OUI to storage vendor mappings.
	// +optional
	// +kubebuilder:default="forklift-naa-oui-map"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	NAAOUIMapConfigMapName string `json:"naa_oui_map_configmap_name,omitempty"`

	// Migration Settings & Timeouts

	// Global default ServiceAccount for migration pods in the target namespace.
	// Overridden by Plan-level serviceAccount.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerMigrationServiceAccount string `json:"controller_migration_service_account,omitempty"`
	// Max concurrent VM migrations.
	// +optional
	// +kubebuilder:default=20
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerMaxVMInflight *int32 `json:"controller_max_vm_inflight,omitempty"`
	// Max concurrent populator pods per ESXi host.
	// +optional
	// +kubebuilder:default=20
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerMaxPopulatorInflight *int32 `json:"controller_max_populator_inflight,omitempty"`
	// Precopy interval in minutes.
	// +optional
	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerPrecopyInterval *int32 `json:"controller_precopy_interval,omitempty"`
	// How long Critical/Error blocker conditions must persist before failing an active migration, in minutes.
	// +optional
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerBlockerGracePeriodMinutes *int32 `json:"controller_blocker_grace_period_minutes,omitempty"`
	// Namespace for host lease objects.
	// +optional
	// +kubebuilder:default="openshift-mtv"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerHostLeaseNamespace string `json:"controller_host_lease_namespace,omitempty"`
	// Host lease duration in seconds.
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerHostLeaseDurationSeconds *int32 `json:"controller_host_lease_duration_seconds,omitempty"`
	// Max concurrent reconciles.
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerMaxConcurrentReconciles *int32 `json:"controller_max_concurrent_reconciles,omitempty"`
	// Snapshot removal timeout in minutes.
	// +optional
	// +kubebuilder:default=120
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerSnapshotRemovalTimeoutMinuts *int32 `json:"controller_snapshot_removal_timeout_minuts,omitempty"`
	// Snapshot status check rate in seconds.
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerSnapshotStatusCheckRateSeconds *int32 `json:"controller_snapshot_status_check_rate_seconds,omitempty"`
	// Cleanup retry count.
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerCleanupRetries *int32 `json:"controller_cleanup_retries,omitempty"`
	// Snapshot removal retries.
	// +optional
	// +kubebuilder:default=20
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerSnapshotRemovalCheckRetries *int32 `json:"controller_snapshot_removal_check_retries,omitempty"`
	// VDDK job timeout in seconds.
	// +optional
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerVDDKJobActiveDeadlineSec *int32 `json:"controller_vddk_job_active_deadline_sec,omitempty"`
	// TLS connection timeout seconds.
	// +optional
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerTLSConnectionTimeoutSec *int32 `json:"controller_tls_connection_timeout_sec,omitempty"`
	// Timeout in seconds for the wait-for-reboot step.
	// +optional
	// +kubebuilder:default=1800
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerWindowsRebootTimeout *int32 `json:"controller_windows_reboot_timeout,omitempty"`
	// Max retries when getting parent disks.
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerMaxParentBackingRetries *int32 `json:"controller_max_parent_backing_retries,omitempty"`

	// Ansible Automation Platform (AAP) Settings

	// Ansible Automation Platform base URL.
	// Required for centralized AAP connection together with aap_token_secret_name.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	AAPUrl string `json:"aap_url,omitempty"`
	// Name of the Secret containing the AAP API Bearer token (data key: token).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	AAPTokenSecretName string `json:"aap_token_secret_name,omitempty"`
	// Default timeout in seconds for AAP HTTP calls and job polling.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	AAPTimeout *int32 `json:"aap_timeout,omitempty"`
	// Skip TLS certificate verification when connecting to AAP.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	AAPInsecureSkipVerify string `json:"aap_insecure_skip_verify,omitempty"`
	// Name of the Secret containing a custom CA certificate (data key: ca.crt) for AAP TLS verification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	AAPCASecretName string `json:"aap_ca_secret_name,omitempty"`

	// Migration Feature-Specific Settings

	// Use vSphere incremental backup.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerVSphereIncrementalBackup string `json:"controller_vsphere_incremental_backup,omitempty"`
	// Enable oVirt warm migration.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerOvirtWarmMigration string `json:"controller_ovirt_warm_migration,omitempty"`
	// Retain precopy pods.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerRetainPrecopyImporterPods string `json:"controller_retain_precopy_importer_pods,omitempty"`
	// Retain populator pods after migration for debugging.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerRetainPopulatorPods string `json:"controller_retain_populator_pods,omitempty"`
	// Ignore xfs_repair exit status during conversion.
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	FeatureXFSRepairIgnore string `json:"feature_xfs_repair_ignore,omitempty"`
	// Enable static UDN IP addresses feature.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerStaticUDNIPAddresses string `json:"controller_static_udn_ip_addresses,omitempty"`
	// Wait for final snapshot removal and consolidation in a VMware warm migration.
	// +optional
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	WaitForFinalSnapshotConsolidation string `json:"wait_for_final_snapshot_consolidation,omitempty"`

	// Storage & Performance Settings

	// Filesystem overhead percentage.
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerFilesystemOverhead *int32 `json:"controller_filesystem_overhead,omitempty"`
	// Block overhead in bytes.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerBlockOverhead *int32 `json:"controller_block_overhead,omitempty"`

	// Virt-v2v Settings

	// Virt-v2v CPU limit.
	// +optional
	// +kubebuilder:default="4000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VContainerLimitsCPU string `json:"virt_v2v_container_limits_cpu,omitempty"`
	// Virt-v2v memory limit.
	// +optional
	// +kubebuilder:default="8Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VContainerLimitsMemory string `json:"virt_v2v_container_limits_memory,omitempty"`
	// Virt-v2v CPU request.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VContainerRequestsCPU string `json:"virt_v2v_container_requests_cpu,omitempty"`
	// Virt-v2v memory request.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VContainerRequestsMemory string `json:"virt_v2v_container_requests_memory,omitempty"`
	// Don't request KVM for virt-v2v. Optional. If left empty, the operator automatically sets this from the environment.
	// +optional
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VDontRequestKVM string `json:"virt_v2v_dont_request_kvm,omitempty"`
	// Additional arguments for virt-v2v conversion. Optional. If left empty, the operator automatically sets this from the environment.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VExtraArgs string `json:"virt_v2v_extra_args,omitempty"`
	// Additional arguments for virt-v2v-inspector. Optional. If left empty, the operator automatically sets this from the environment.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VInspectorExtraArgs string `json:"virt_v2v_inspector_extra_args,omitempty"`
	// ConfigMap name containing extra virt-v2v configuration. Optional. If left empty, the operator automatically sets this from the environment.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VExtraConfConfigMap string `json:"virt_v2v_extra_conf_config_map,omitempty"`
	// Memory (in MB) allocated for the virt-v2v conversion appliance.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VMemsize *int32 `json:"virt_v2v_memsize,omitempty"`
	// Number of virtual CPUs for the virt-v2v conversion appliance.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	VirtV2VSMP *int32 `json:"virt_v2v_smp,omitempty"`

	// Hooks Settings

	// Hooks CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HooksContainerLimitsCPU string `json:"hooks_container_limits_cpu,omitempty"`
	// Hooks memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HooksContainerLimitsMemory string `json:"hooks_container_limits_memory,omitempty"`
	// Hooks CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HooksContainerRequestsCPU string `json:"hooks_container_requests_cpu,omitempty"`
	// Hooks memory request.
	// +optional
	// +kubebuilder:default="150Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HooksContainerRequestsMemory string `json:"hooks_container_requests_memory,omitempty"`

	// OVA Settings

	// OVA CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAContainerLimitsCPU string `json:"ova_container_limits_cpu,omitempty"`
	// OVA memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAContainerLimitsMemory string `json:"ova_container_limits_memory,omitempty"`
	// OVA CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAContainerRequestsCPU string `json:"ova_container_requests_cpu,omitempty"`
	// OVA memory request.
	// +optional
	// +kubebuilder:default="512Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAContainerRequestsMemory string `json:"ova_container_requests_memory,omitempty"`

	// HyperV Settings

	// HyperV CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HyperVContainerLimitsCPU string `json:"hyperv_container_limits_cpu,omitempty"`
	// HyperV memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HyperVContainerLimitsMemory string `json:"hyperv_container_limits_memory,omitempty"`
	// HyperV CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HyperVContainerRequestsCPU string `json:"hyperv_container_requests_cpu,omitempty"`
	// HyperV memory request.
	// +optional
	// +kubebuilder:default="512Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	HyperVContainerRequestsMemory string `json:"hyperv_container_requests_memory,omitempty"`
	// HyperV inventory refresh interval as a Go duration.
	// +optional
	// +kubebuilder:default="10s"
	// +kubebuilder:validation:Pattern="^[0-9]+(s|m|h)$"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerHyperVRefreshInterval string `json:"controller_hyperv_refresh_interval,omitempty"`
	// Timeout for HyperV SMB disk validation HTTP calls as a Go duration.
	// +optional
	// +kubebuilder:default="30s"
	// +kubebuilder:validation:Pattern="^[0-9]+(s|m|h)$"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerHyperVValidationTimeout string `json:"controller_hyperv_validation_timeout,omitempty"`

	// OVA Proxy Settings

	// OVA Proxy CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProxyContainerLimitsCPU string `json:"ova_proxy_container_limits_cpu,omitempty"`
	// OVA Proxy memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProxyContainerLimitsMemory string `json:"ova_proxy_container_limits_memory,omitempty"`
	// OVA Proxy CPU request.
	// +optional
	// +kubebuilder:default="250m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProxyContainerRequestsCPU string `json:"ova_proxy_container_requests_cpu,omitempty"`
	// OVA Proxy memory request.
	// +optional
	// +kubebuilder:default="512Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProxyContainerRequestsMemory string `json:"ova_proxy_container_requests_memory,omitempty"`
	// OVA Proxy route timeout.
	// +optional
	// +kubebuilder:default="360s"
	// +kubebuilder:validation:Pattern="^[0-9]+(s|m|h)$"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OVAProxyRouteTimeout string `json:"ova_proxy_route_timeout,omitempty"`

	// Volume Populator Settings

	// Volume Populator CPU limit.
	// +optional
	// +kubebuilder:default="1000m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorContainerLimitsCPU string `json:"populator_container_limits_cpu,omitempty"`
	// Volume Populator memory limit.
	// +optional
	// +kubebuilder:default="1Gi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorContainerLimitsMemory string `json:"populator_container_limits_memory,omitempty"`
	// Volume Populator CPU request.
	// +optional
	// +kubebuilder:default="100m"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorContainerRequestsCPU string `json:"populator_container_requests_cpu,omitempty"`
	// Volume Populator memory request.
	// +optional
	// +kubebuilder:default="512Mi"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	PopulatorContainerRequestsMemory string `json:"populator_container_requests_memory,omitempty"`

	// Logging & General Settings

	// Log verbosity level.
	// +optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ControllerLogLevel *int32 `json:"controller_log_level,omitempty"`
	// Image pull policy.
	// +optional
	// +kubebuilder:default="Always"
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	ImagePullPolicy string `json:"image_pull_policy,omitempty"`
	// Whether running on Kubernetes (vs OpenShift).
	// +optional
	// +kubebuilder:default="false"
	// +kubebuilder:validation:Enum="true";"false"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	K8sCluster string `json:"k8s_cluster,omitempty"`

	// Metrics Settings

	// Metrics scrape interval.
	// +optional
	// +kubebuilder:default="30s"
	// +kubebuilder:validation:Pattern="^[0-9]+(s|m|h)$"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	MetricInterval string `json:"metric_interval,omitempty"`

	// OLM Metadata

	// Set by the OLM subscription lifecycle to indicate the CR is managed by OLM.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	OlmManaged *bool `json:"olm_managed,omitempty"`
}

// ForkliftControllerStatus defines the observed state of ForkliftController.
// Status is managed by the Ansible operator and may contain arbitrary fields
// including conditions with Ansible-specific metadata.
// +kubebuilder:validation:XPreserveUnknownFields
type ForkliftControllerStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status

// ForkliftController is the Schema for the forkliftcontrollers API.
type ForkliftController struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ForkliftControllerSpec   `json:"spec,omitempty"`
	Status          ForkliftControllerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ForkliftControllerList contains a list of ForkliftController.
type ForkliftControllerList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []ForkliftController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ForkliftController{}, &ForkliftControllerList{})
}
