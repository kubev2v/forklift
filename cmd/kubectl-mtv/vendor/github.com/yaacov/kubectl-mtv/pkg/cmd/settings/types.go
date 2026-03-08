// Package settings provides types and utilities for managing ForkliftController settings.
package settings

import "sort"

// SettingType represents the data type of a setting.
type SettingType string

const (
	// TypeString represents a string setting.
	TypeString SettingType = "string"
	// TypeBool represents a boolean setting.
	TypeBool SettingType = "bool"
	// TypeInt represents an integer setting.
	TypeInt SettingType = "int"
)

// SettingCategory represents the category of a setting.
type SettingCategory string

const (
	// CategoryImage represents container image settings.
	CategoryImage SettingCategory = "image"
	// CategoryFeature represents feature flag settings.
	CategoryFeature SettingCategory = "feature"
	// CategoryPerformance represents performance tuning settings.
	CategoryPerformance SettingCategory = "performance"
	// CategoryDebug represents debugging settings.
	CategoryDebug SettingCategory = "debug"
	// CategoryVirtV2V represents virt-v2v container settings.
	CategoryVirtV2V SettingCategory = "virt-v2v"
	// CategoryPopulator represents volume populator container settings.
	CategoryPopulator SettingCategory = "populator"
	// CategoryHook represents hook container settings.
	CategoryHook SettingCategory = "hook"
	// CategoryOVA represents OVA provider server container settings.
	CategoryOVA SettingCategory = "ova"
	// CategoryHyperV represents HyperV provider server container settings.
	CategoryHyperV SettingCategory = "hyperv"
	// CategoryController represents controller deployment resource settings.
	CategoryController SettingCategory = "controller"
	// CategoryInventory represents inventory container resource settings.
	CategoryInventory SettingCategory = "inventory"
	// CategoryAPI represents API service resource settings.
	CategoryAPI SettingCategory = "api"
	// CategoryUIPlugin represents UI plugin resource settings.
	CategoryUIPlugin SettingCategory = "ui-plugin"
	// CategoryValidation represents validation service resource settings.
	CategoryValidation SettingCategory = "validation"
	// CategoryCLIDownload represents CLI download service resource settings.
	CategoryCLIDownload SettingCategory = "cli-download"
	// CategoryOVAProxy represents OVA proxy resource settings.
	CategoryOVAProxy SettingCategory = "ova-proxy"
	// CategoryConfigMaps represents ConfigMap name settings.
	CategoryConfigMaps SettingCategory = "configmaps"
	// CategoryAdvanced represents advanced/misc settings.
	CategoryAdvanced SettingCategory = "advanced"
)

// SettingDefinition defines metadata for a ForkliftController setting.
type SettingDefinition struct {
	// Name is the setting key in the ForkliftController spec (snake_case).
	Name string
	// Type is the data type of the setting.
	Type SettingType
	// Default is the default value if not set.
	Default interface{}
	// Description is a human-readable description of the setting.
	Description string
	// Category groups related settings together.
	Category SettingCategory
}

// SettingValue represents a setting with its current and default values.
type SettingValue struct {
	// Name is the setting key.
	Name string
	// Value is the current value (nil if not set).
	Value interface{}
	// Default is the default value.
	Default interface{}
	// IsSet indicates whether the value is explicitly set.
	IsSet bool
	// Definition contains the setting metadata.
	Definition SettingDefinition
}

// CategoryOrder defines the display order for categories.
var CategoryOrder = []SettingCategory{
	CategoryImage,
	CategoryFeature,
	CategoryPerformance,
	CategoryDebug,
	CategoryVirtV2V,
	CategoryPopulator,
	CategoryHook,
	CategoryOVA,
	CategoryHyperV,
	CategoryController,
	CategoryInventory,
	CategoryAPI,
	CategoryUIPlugin,
	CategoryValidation,
	CategoryCLIDownload,
	CategoryOVAProxy,
	CategoryConfigMaps,
	CategoryAdvanced,
}

// SupportedSettings contains all supported ForkliftController settings.
// This is a curated subset of settings that users commonly need to configure.
var SupportedSettings = map[string]SettingDefinition{
	// Container Images
	"vddk_image": {
		Name:        "vddk_image",
		Type:        TypeString,
		Default:     "",
		Description: "VDDK container image for vSphere migrations",
		Category:    CategoryImage,
	},
	"virt_v2v_image_fqin": {
		Name:        "virt_v2v_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "Custom virt-v2v container image",
		Category:    CategoryImage,
	},

	// Feature Flags
	"controller_vsphere_incremental_backup": {
		Name:        "controller_vsphere_incremental_backup",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable CBT-based warm migration for vSphere",
		Category:    CategoryFeature,
	},
	"controller_ovirt_warm_migration": {
		Name:        "controller_ovirt_warm_migration",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable warm migration from oVirt",
		Category:    CategoryFeature,
	},
	"feature_copy_offload": {
		Name:        "feature_copy_offload",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable storage array offload (XCOPY)",
		Category:    CategoryFeature,
	},
	"feature_ocp_live_migration": {
		Name:        "feature_ocp_live_migration",
		Type:        TypeBool,
		Default:     false,
		Description: "Enable cross-cluster OpenShift live migration",
		Category:    CategoryFeature,
	},
	"feature_vmware_system_serial_number": {
		Name:        "feature_vmware_system_serial_number",
		Type:        TypeBool,
		Default:     true,
		Description: "Use VMware system serial number for migrated VMs",
		Category:    CategoryFeature,
	},
	"controller_static_udn_ip_addresses": {
		Name:        "controller_static_udn_ip_addresses",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable static IP addresses with User Defined Networks",
		Category:    CategoryFeature,
	},
	"controller_retain_precopy_importer_pods": {
		Name:        "controller_retain_precopy_importer_pods",
		Type:        TypeBool,
		Default:     false,
		Description: "Retain importer pods during warm migration (debugging)",
		Category:    CategoryFeature,
	},
	"feature_ova_appliance_management": {
		Name:        "feature_ova_appliance_management",
		Type:        TypeBool,
		Default:     false,
		Description: "Enable appliance management for OVF-based providers",
		Category:    CategoryFeature,
	},

	// Performance Tuning
	"controller_max_vm_inflight": {
		Name:        "controller_max_vm_inflight",
		Type:        TypeInt,
		Default:     20,
		Description: "Maximum concurrent VM migrations",
		Category:    CategoryPerformance,
	},
	"controller_precopy_interval": {
		Name:        "controller_precopy_interval",
		Type:        TypeInt,
		Default:     60,
		Description: "Minutes between warm migration precopies",
		Category:    CategoryPerformance,
	},
	"controller_max_concurrent_reconciles": {
		Name:        "controller_max_concurrent_reconciles",
		Type:        TypeInt,
		Default:     10,
		Description: "Maximum concurrent controller reconciles",
		Category:    CategoryPerformance,
	},
	"controller_snapshot_removal_timeout_minuts": {
		Name:        "controller_snapshot_removal_timeout_minuts",
		Type:        TypeInt,
		Default:     120,
		Description: "Timeout for snapshot removal (minutes)",
		Category:    CategoryPerformance,
	},
	"controller_vddk_job_active_deadline_sec": {
		Name:        "controller_vddk_job_active_deadline_sec",
		Type:        TypeInt,
		Default:     300,
		Description: "VDDK validation job deadline (seconds)",
		Category:    CategoryPerformance,
	},
	"controller_filesystem_overhead": {
		Name:        "controller_filesystem_overhead",
		Type:        TypeInt,
		Default:     10,
		Description: "Filesystem overhead percentage",
		Category:    CategoryPerformance,
	},
	"controller_block_overhead": {
		Name:        "controller_block_overhead",
		Type:        TypeInt,
		Default:     0,
		Description: "Block storage fixed overhead (bytes)",
		Category:    CategoryPerformance,
	},
	"controller_cleanup_retries": {
		Name:        "controller_cleanup_retries",
		Type:        TypeInt,
		Default:     10,
		Description: "Maximum cleanup retry attempts",
		Category:    CategoryPerformance,
	},
	"controller_snapshot_removal_check_retries": {
		Name:        "controller_snapshot_removal_check_retries",
		Type:        TypeInt,
		Default:     20,
		Description: "Maximum snapshot removal check retries",
		Category:    CategoryPerformance,
	},
	"controller_host_lease_namespace": {
		Name:        "controller_host_lease_namespace",
		Type:        TypeString,
		Default:     "openshift-mtv",
		Description: "Namespace for host lease objects (copy offload)",
		Category:    CategoryPerformance,
	},
	"controller_host_lease_duration_seconds": {
		Name:        "controller_host_lease_duration_seconds",
		Type:        TypeInt,
		Default:     10,
		Description: "Host lease duration in seconds (copy offload)",
		Category:    CategoryPerformance,
	},

	// Debugging
	"controller_log_level": {
		Name:        "controller_log_level",
		Type:        TypeInt,
		Default:     3,
		Description: "Controller log verbosity (0-9)",
		Category:    CategoryDebug,
	},

	// virt-v2v Container Settings
	"virt_v2v_extra_args": {
		Name:        "virt_v2v_extra_args",
		Type:        TypeString,
		Default:     "",
		Description: "Additional virt-v2v command-line arguments",
		Category:    CategoryVirtV2V,
	},
	"virt_v2v_dont_request_kvm": {
		Name:        "virt_v2v_dont_request_kvm",
		Type:        TypeBool,
		Default:     false,
		Description: "Don't request KVM device (use for nested virtualization)",
		Category:    CategoryVirtV2V,
	},
	"virt_v2v_extra_conf_config_map": {
		Name:        "virt_v2v_extra_conf_config_map",
		Type:        TypeString,
		Default:     "",
		Description: "ConfigMap with extra virt-v2v configuration files",
		Category:    CategoryVirtV2V,
	},
	"virt_v2v_container_limits_cpu": {
		Name:        "virt_v2v_container_limits_cpu",
		Type:        TypeString,
		Default:     "4000m",
		Description: "virt-v2v container CPU limit",
		Category:    CategoryVirtV2V,
	},
	"virt_v2v_container_limits_memory": {
		Name:        "virt_v2v_container_limits_memory",
		Type:        TypeString,
		Default:     "8Gi",
		Description: "virt-v2v container memory limit",
		Category:    CategoryVirtV2V,
	},
	"virt_v2v_container_requests_cpu": {
		Name:        "virt_v2v_container_requests_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "virt-v2v container CPU request",
		Category:    CategoryVirtV2V,
	},
	"virt_v2v_container_requests_memory": {
		Name:        "virt_v2v_container_requests_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "virt-v2v container memory request",
		Category:    CategoryVirtV2V,
	},

	// Volume Populator Container Settings
	"populator_container_limits_cpu": {
		Name:        "populator_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "Volume populator container CPU limit",
		Category:    CategoryPopulator,
	},
	"populator_container_limits_memory": {
		Name:        "populator_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "Volume populator container memory limit",
		Category:    CategoryPopulator,
	},
	"populator_container_requests_cpu": {
		Name:        "populator_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "Volume populator container CPU request",
		Category:    CategoryPopulator,
	},
	"populator_container_requests_memory": {
		Name:        "populator_container_requests_memory",
		Type:        TypeString,
		Default:     "512Mi",
		Description: "Volume populator container memory request",
		Category:    CategoryPopulator,
	},

	// Hook Container Settings
	"hooks_container_limits_cpu": {
		Name:        "hooks_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "Hook container CPU limit",
		Category:    CategoryHook,
	},
	"hooks_container_limits_memory": {
		Name:        "hooks_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "Hook container memory limit",
		Category:    CategoryHook,
	},
	"hooks_container_requests_cpu": {
		Name:        "hooks_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "Hook container CPU request",
		Category:    CategoryHook,
	},
	"hooks_container_requests_memory": {
		Name:        "hooks_container_requests_memory",
		Type:        TypeString,
		Default:     "150Mi",
		Description: "Hook container memory request",
		Category:    CategoryHook,
	},

	// OVA Provider Server Container Settings
	"ova_container_limits_cpu": {
		Name:        "ova_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "OVA provider server CPU limit",
		Category:    CategoryOVA,
	},
	"ova_container_limits_memory": {
		Name:        "ova_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "OVA provider server memory limit",
		Category:    CategoryOVA,
	},
	"ova_container_requests_cpu": {
		Name:        "ova_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "OVA provider server CPU request",
		Category:    CategoryOVA,
	},
	"ova_container_requests_memory": {
		Name:        "ova_container_requests_memory",
		Type:        TypeString,
		Default:     "512Mi",
		Description: "OVA provider server memory request",
		Category:    CategoryOVA,
	},

	// HyperV Provider Server Container Settings
	"hyperv_container_limits_cpu": {
		Name:        "hyperv_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "HyperV provider server CPU limit",
		Category:    CategoryHyperV,
	},
	"hyperv_container_limits_memory": {
		Name:        "hyperv_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "HyperV provider server memory limit",
		Category:    CategoryHyperV,
	},
	"hyperv_container_requests_cpu": {
		Name:        "hyperv_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "HyperV provider server CPU request",
		Category:    CategoryHyperV,
	},
	"hyperv_container_requests_memory": {
		Name:        "hyperv_container_requests_memory",
		Type:        TypeString,
		Default:     "512Mi",
		Description: "HyperV provider server memory request",
		Category:    CategoryHyperV,
	},
}

// ExtendedSettings contains additional ForkliftController settings not in the curated SupportedSettings.
// These are less commonly used settings that are available via the --all flag.
var ExtendedSettings = map[string]SettingDefinition{
	// Additional Feature Gates
	"feature_ui_plugin": {
		Name:        "feature_ui_plugin",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable OpenShift Console UI plugin",
		Category:    CategoryFeature,
	},
	"feature_validation": {
		Name:        "feature_validation",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable VM validation with OPA policies",
		Category:    CategoryFeature,
	},
	"feature_volume_populator": {
		Name:        "feature_volume_populator",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable volume populator for oVirt/OpenStack",
		Category:    CategoryFeature,
	},
	"feature_auth_required": {
		Name:        "feature_auth_required",
		Type:        TypeBool,
		Default:     true,
		Description: "Require authentication for inventory API",
		Category:    CategoryFeature,
	},
	"feature_cli_download": {
		Name:        "feature_cli_download",
		Type:        TypeBool,
		Default:     true,
		Description: "Enable kubectl-mtv CLI download service",
		Category:    CategoryFeature,
	},

	// Controller Deployment Resources
	"controller_container_limits_cpu": {
		Name:        "controller_container_limits_cpu",
		Type:        TypeString,
		Default:     "2",
		Description: "Controller container CPU limit",
		Category:    CategoryController,
	},
	"controller_container_limits_memory": {
		Name:        "controller_container_limits_memory",
		Type:        TypeString,
		Default:     "800Mi",
		Description: "Controller container memory limit",
		Category:    CategoryController,
	},
	"controller_container_requests_cpu": {
		Name:        "controller_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "Controller container CPU request",
		Category:    CategoryController,
	},
	"controller_container_requests_memory": {
		Name:        "controller_container_requests_memory",
		Type:        TypeString,
		Default:     "350Mi",
		Description: "Controller container memory request",
		Category:    CategoryController,
	},
	"controller_transfer_network": {
		Name:        "controller_transfer_network",
		Type:        TypeString,
		Default:     "",
		Description: "Optional NAD name for controller pod transfer network (format: namespace/network-name)",
		Category:    CategoryController,
	},

	// Inventory Container Resources
	"inventory_container_limits_cpu": {
		Name:        "inventory_container_limits_cpu",
		Type:        TypeString,
		Default:     "2",
		Description: "Inventory container CPU limit",
		Category:    CategoryInventory,
	},
	"inventory_container_limits_memory": {
		Name:        "inventory_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "Inventory container memory limit",
		Category:    CategoryInventory,
	},
	"inventory_container_requests_cpu": {
		Name:        "inventory_container_requests_cpu",
		Type:        TypeString,
		Default:     "500m",
		Description: "Inventory container CPU request",
		Category:    CategoryInventory,
	},
	"inventory_container_requests_memory": {
		Name:        "inventory_container_requests_memory",
		Type:        TypeString,
		Default:     "500Mi",
		Description: "Inventory container memory request",
		Category:    CategoryInventory,
	},
	"inventory_route_timeout": {
		Name:        "inventory_route_timeout",
		Type:        TypeString,
		Default:     "360s",
		Description: "Inventory route timeout",
		Category:    CategoryInventory,
	},

	// API Service Resources
	"api_container_limits_cpu": {
		Name:        "api_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "API service CPU limit",
		Category:    CategoryAPI,
	},
	"api_container_limits_memory": {
		Name:        "api_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "API service memory limit",
		Category:    CategoryAPI,
	},
	"api_container_requests_cpu": {
		Name:        "api_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "API service CPU request",
		Category:    CategoryAPI,
	},
	"api_container_requests_memory": {
		Name:        "api_container_requests_memory",
		Type:        TypeString,
		Default:     "150Mi",
		Description: "API service memory request",
		Category:    CategoryAPI,
	},

	// UI Plugin Resources
	"ui_plugin_container_limits_cpu": {
		Name:        "ui_plugin_container_limits_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "UI plugin CPU limit",
		Category:    CategoryUIPlugin,
	},
	"ui_plugin_container_limits_memory": {
		Name:        "ui_plugin_container_limits_memory",
		Type:        TypeString,
		Default:     "800Mi",
		Description: "UI plugin memory limit",
		Category:    CategoryUIPlugin,
	},
	"ui_plugin_container_requests_cpu": {
		Name:        "ui_plugin_container_requests_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "UI plugin CPU request",
		Category:    CategoryUIPlugin,
	},
	"ui_plugin_container_requests_memory": {
		Name:        "ui_plugin_container_requests_memory",
		Type:        TypeString,
		Default:     "150Mi",
		Description: "UI plugin memory request",
		Category:    CategoryUIPlugin,
	},

	// Validation Service Resources
	"validation_container_limits_cpu": {
		Name:        "validation_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "Validation service CPU limit",
		Category:    CategoryValidation,
	},
	"validation_container_limits_memory": {
		Name:        "validation_container_limits_memory",
		Type:        TypeString,
		Default:     "300Mi",
		Description: "Validation service memory limit",
		Category:    CategoryValidation,
	},
	"validation_container_requests_cpu": {
		Name:        "validation_container_requests_cpu",
		Type:        TypeString,
		Default:     "400m",
		Description: "Validation service CPU request",
		Category:    CategoryValidation,
	},
	"validation_container_requests_memory": {
		Name:        "validation_container_requests_memory",
		Type:        TypeString,
		Default:     "50Mi",
		Description: "Validation service memory request",
		Category:    CategoryValidation,
	},
	"validation_policy_agent_search_interval": {
		Name:        "validation_policy_agent_search_interval",
		Type:        TypeInt,
		Default:     120,
		Description: "Policy agent search interval in seconds",
		Category:    CategoryValidation,
	},
	"validation_extra_volume_name": {
		Name:        "validation_extra_volume_name",
		Type:        TypeString,
		Default:     "validation-extra-rules",
		Description: "Volume name for extra validation rules",
		Category:    CategoryValidation,
	},
	"validation_extra_volume_mountpath": {
		Name:        "validation_extra_volume_mountpath",
		Type:        TypeString,
		Default:     "/usr/share/opa/policies/extra",
		Description: "Mount path for extra validation rules",
		Category:    CategoryValidation,
	},

	// CLI Download Service Resources
	"cli_download_container_limits_cpu": {
		Name:        "cli_download_container_limits_cpu",
		Type:        TypeString,
		Default:     "100m",
		Description: "CLI download service CPU limit",
		Category:    CategoryCLIDownload,
	},
	"cli_download_container_limits_memory": {
		Name:        "cli_download_container_limits_memory",
		Type:        TypeString,
		Default:     "128Mi",
		Description: "CLI download service memory limit",
		Category:    CategoryCLIDownload,
	},
	"cli_download_container_requests_cpu": {
		Name:        "cli_download_container_requests_cpu",
		Type:        TypeString,
		Default:     "50m",
		Description: "CLI download service CPU request",
		Category:    CategoryCLIDownload,
	},
	"cli_download_container_requests_memory": {
		Name:        "cli_download_container_requests_memory",
		Type:        TypeString,
		Default:     "64Mi",
		Description: "CLI download service memory request",
		Category:    CategoryCLIDownload,
	},

	// OVA Proxy Resources
	"ova_proxy_container_limits_cpu": {
		Name:        "ova_proxy_container_limits_cpu",
		Type:        TypeString,
		Default:     "1000m",
		Description: "OVA proxy CPU limit",
		Category:    CategoryOVAProxy,
	},
	"ova_proxy_container_limits_memory": {
		Name:        "ova_proxy_container_limits_memory",
		Type:        TypeString,
		Default:     "1Gi",
		Description: "OVA proxy memory limit",
		Category:    CategoryOVAProxy,
	},
	"ova_proxy_container_requests_cpu": {
		Name:        "ova_proxy_container_requests_cpu",
		Type:        TypeString,
		Default:     "250m",
		Description: "OVA proxy CPU request",
		Category:    CategoryOVAProxy,
	},
	"ova_proxy_container_requests_memory": {
		Name:        "ova_proxy_container_requests_memory",
		Type:        TypeString,
		Default:     "512Mi",
		Description: "OVA proxy memory request",
		Category:    CategoryOVAProxy,
	},
	"ova_proxy_route_timeout": {
		Name:        "ova_proxy_route_timeout",
		Type:        TypeString,
		Default:     "360s",
		Description: "OVA proxy route timeout",
		Category:    CategoryOVAProxy,
	},

	// Additional Container Images (FQINs)
	"controller_image_fqin": {
		Name:        "controller_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "Controller pod image",
		Category:    CategoryImage,
	},
	"api_image_fqin": {
		Name:        "api_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "API service image",
		Category:    CategoryImage,
	},
	"validation_image_fqin": {
		Name:        "validation_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "Validation service image",
		Category:    CategoryImage,
	},
	"ui_plugin_image_fqin": {
		Name:        "ui_plugin_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "UI plugin image",
		Category:    CategoryImage,
	},
	"cli_download_image_fqin": {
		Name:        "cli_download_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "CLI download service image",
		Category:    CategoryImage,
	},
	"populator_controller_image_fqin": {
		Name:        "populator_controller_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "Volume populator controller image",
		Category:    CategoryImage,
	},
	"populator_ovirt_image_fqin": {
		Name:        "populator_ovirt_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "oVirt populator image",
		Category:    CategoryImage,
	},
	"populator_openstack_image_fqin": {
		Name:        "populator_openstack_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "OpenStack populator image",
		Category:    CategoryImage,
	},
	"populator_vsphere_xcopy_volume_image_fqin": {
		Name:        "populator_vsphere_xcopy_volume_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "vSphere xcopy populator image",
		Category:    CategoryImage,
	},
	"ova_provider_server_fqin": {
		Name:        "ova_provider_server_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "OVA provider server image",
		Category:    CategoryImage,
	},
	"hyperv_provider_server_fqin": {
		Name:        "hyperv_provider_server_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "HyperV provider server image",
		Category:    CategoryImage,
	},
	"must_gather_image_fqin": {
		Name:        "must_gather_image_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "Must-gather debugging image",
		Category:    CategoryImage,
	},
	"ova_proxy_fqin": {
		Name:        "ova_proxy_fqin",
		Type:        TypeString,
		Default:     "",
		Description: "OVA inventory proxy image",
		Category:    CategoryImage,
	},

	// ConfigMap Names
	"ovirt_osmap_configmap_name": {
		Name:        "ovirt_osmap_configmap_name",
		Type:        TypeString,
		Default:     "forklift-ovirt-osmap",
		Description: "ConfigMap name for oVirt OS mappings",
		Category:    CategoryConfigMaps,
	},
	"vsphere_osmap_configmap_name": {
		Name:        "vsphere_osmap_configmap_name",
		Type:        TypeString,
		Default:     "forklift-vsphere-osmap",
		Description: "ConfigMap name for vSphere OS mappings",
		Category:    CategoryConfigMaps,
	},
	"virt_customize_configmap_name": {
		Name:        "virt_customize_configmap_name",
		Type:        TypeString,
		Default:     "forklift-virt-customize",
		Description: "ConfigMap name for virt-customize configuration",
		Category:    CategoryConfigMaps,
	},

	// Advanced Settings
	"controller_snapshot_status_check_rate_seconds": {
		Name:        "controller_snapshot_status_check_rate_seconds",
		Type:        TypeInt,
		Default:     10,
		Description: "Rate for checking snapshot status (seconds)",
		Category:    CategoryAdvanced,
	},
	"controller_tls_connection_timeout_sec": {
		Name:        "controller_tls_connection_timeout_sec",
		Type:        TypeInt,
		Default:     5,
		Description: "TLS connection timeout (seconds)",
		Category:    CategoryAdvanced,
	},
	"controller_max_parent_backing_retries": {
		Name:        "controller_max_parent_backing_retries",
		Type:        TypeInt,
		Default:     10,
		Description: "Maximum retries for parent backing lookup",
		Category:    CategoryAdvanced,
	},
	"controller_cdi_export_token_ttl": {
		Name:        "controller_cdi_export_token_ttl",
		Type:        TypeInt,
		Default:     720,
		Description: "CDI export token TTL (minutes)",
		Category:    CategoryAdvanced,
	},
	"image_pull_policy": {
		Name:        "image_pull_policy",
		Type:        TypeString,
		Default:     "Always",
		Description: "Image pull policy (Always, IfNotPresent, Never)",
		Category:    CategoryAdvanced,
	},
	"k8s_cluster": {
		Name:        "k8s_cluster",
		Type:        TypeBool,
		Default:     false,
		Description: "Whether running on Kubernetes (vs OpenShift)",
		Category:    CategoryAdvanced,
	},
	"metric_interval": {
		Name:        "metric_interval",
		Type:        TypeString,
		Default:     "30s",
		Description: "Metrics scrape interval",
		Category:    CategoryAdvanced,
	},
}

// GetAllSettings returns a merged map of SupportedSettings + ExtendedSettings.
// This provides access to all known ForkliftController settings.
func GetAllSettings() map[string]SettingDefinition {
	all := make(map[string]SettingDefinition, len(SupportedSettings)+len(ExtendedSettings))
	for k, v := range SupportedSettings {
		all[k] = v
	}
	for k, v := range ExtendedSettings {
		all[k] = v
	}
	return all
}

// GetAllSettingNames returns all setting names (supported + extended) in a consistent order.
func GetAllSettingNames() []string {
	allSettings := GetAllSettings()
	var names []string
	for _, category := range CategoryOrder {
		var categoryNames []string
		for name, def := range allSettings {
			if def.Category == category {
				categoryNames = append(categoryNames, name)
			}
		}
		sort.Strings(categoryNames)
		names = append(names, categoryNames...)
	}
	return names
}

// GetSettingNames returns all supported setting names in a consistent order.
func GetSettingNames() []string {
	var names []string
	for _, category := range CategoryOrder {
		// Collect names for this category
		var categoryNames []string
		for name, def := range SupportedSettings {
			if def.Category == category {
				categoryNames = append(categoryNames, name)
			}
		}
		// Sort names within category for deterministic ordering
		sort.Strings(categoryNames)
		names = append(names, categoryNames...)
	}
	return names
}

// GetSettingsByCategory returns settings grouped by category.
func GetSettingsByCategory() map[SettingCategory][]SettingDefinition {
	result := make(map[SettingCategory][]SettingDefinition)
	for _, def := range SupportedSettings {
		result[def.Category] = append(result[def.Category], def)
	}
	return result
}

// IsValidSetting checks if a setting name is in the curated SupportedSettings.
func IsValidSetting(name string) bool {
	_, ok := SupportedSettings[name]
	return ok
}

// IsValidAnySetting checks if a setting name is valid (in either SupportedSettings or ExtendedSettings).
func IsValidAnySetting(name string) bool {
	if _, ok := SupportedSettings[name]; ok {
		return true
	}
	_, ok := ExtendedSettings[name]
	return ok
}

// GetSettingDefinition returns the definition for a setting from SupportedSettings, or nil if not found.
func GetSettingDefinition(name string) *SettingDefinition {
	if def, ok := SupportedSettings[name]; ok {
		return &def
	}
	return nil
}

// GetAnySettingDefinition returns the definition for a setting from all settings, or nil if not found.
func GetAnySettingDefinition(name string) *SettingDefinition {
	if def, ok := SupportedSettings[name]; ok {
		return &def
	}
	if def, ok := ExtendedSettings[name]; ok {
		return &def
	}
	return nil
}
