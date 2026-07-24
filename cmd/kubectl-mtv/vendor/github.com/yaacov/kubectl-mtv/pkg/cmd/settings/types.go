// Package settings provides types and utilities for managing ForkliftController settings.
package settings

import "sort"

// SettingType represents the data type of a setting.
type SettingType string

const (
	TypeString SettingType = "string"
	TypeBool   SettingType = "bool"
	TypeInt    SettingType = "int"
)

// SettingCategory represents the category of a setting.
type SettingCategory string

const (
	CategoryImage       SettingCategory = "image"
	CategoryFeature     SettingCategory = "feature"
	CategoryPerformance SettingCategory = "performance"
	CategoryDebug       SettingCategory = "debug"
	CategoryVirtV2V     SettingCategory = "virt-v2v"
	CategoryPopulator   SettingCategory = "populator"
	CategoryHook        SettingCategory = "hook"
	CategoryOVA         SettingCategory = "ova"
	CategoryHyperV      SettingCategory = "hyperv"
	CategoryController  SettingCategory = "controller"
	CategoryInventory   SettingCategory = "inventory"
	CategoryAPI         SettingCategory = "api"
	CategoryUIPlugin    SettingCategory = "ui-plugin"
	CategoryValidation  SettingCategory = "validation"
	CategoryCLIDownload SettingCategory = "cli-download"
	CategoryMCP         SettingCategory = "mcp-server"
	CategoryOVAProxy    SettingCategory = "ova-proxy"
	CategoryConfigMaps  SettingCategory = "configmaps"
	CategoryAdvanced    SettingCategory = "advanced"
	CategoryAAP         SettingCategory = "aap"
)

// SettingDefinition defines metadata for a ForkliftController setting.
type SettingDefinition struct {
	Name        string
	Type        SettingType
	Default     interface{}
	Description string
	Category    SettingCategory
}

// SettingValue represents a setting with its current and default values.
type SettingValue struct {
	Name       string
	Value      interface{}
	Default    interface{}
	IsSet      bool
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
	CategoryMCP,
	CategoryOVAProxy,
	CategoryConfigMaps,
	CategoryAdvanced,
	CategoryAAP,
}

// SupportedSettingNames is the curated list of settings shown by default (without --all).
// These are the most commonly configured ForkliftController settings.
var SupportedSettingNames = []string{
	// Images
	"vddk_image",
	"virt_v2v_image_fqin",
	"populator_vsphere_copy_offload_image_fqin",

	// Feature flags
	"controller_vsphere_incremental_backup",
	"controller_ovirt_warm_migration",
	"feature_copy_offload",
	"feature_ocp_live_migration",
	"controller_static_udn_ip_addresses",
	"controller_retain_precopy_importer_pods",
	"controller_retain_populator_pods",
	"feature_ova_appliance_management",
	"feature_vsphere_vmware_driver_removal",
	"feature_windows_registry_network_config",
	"feature_windows_wait_for_reboot",
	"feature_mcp_server",

	// Performance
	"controller_max_vm_inflight",
	"controller_precopy_interval",
	"controller_max_concurrent_reconciles",
	"controller_snapshot_removal_timeout_minuts",
	"controller_vddk_job_active_deadline_sec",
	"controller_windows_reboot_timeout",
	"controller_max_populator_inflight",
	"controller_filesystem_overhead",
	"controller_block_overhead",
	"controller_cleanup_retries",
	"controller_blocker_grace_period_minutes",
	"controller_snapshot_removal_check_retries",
	"controller_host_lease_namespace",
	"controller_host_lease_duration_seconds",

	// Debug
	"controller_log_level",

	// Virt-v2v
	"virt_v2v_extra_args",
	"virt_v2v_dont_request_kvm",
	"virt_v2v_extra_conf_config_map",
	"virt_v2v_inspector_extra_args",
	"virt_v2v_memsize",
	"virt_v2v_smp",
	"virt_v2v_container_limits_cpu",
	"virt_v2v_container_limits_memory",
	"virt_v2v_container_requests_cpu",
	"virt_v2v_container_requests_memory",

	// Populator
	"populator_container_limits_cpu",
	"populator_container_limits_memory",
	"populator_container_requests_cpu",
	"populator_container_requests_memory",

	// Hooks
	"hooks_container_limits_cpu",
	"hooks_container_limits_memory",
	"hooks_container_requests_cpu",
	"hooks_container_requests_memory",

	// OVA
	"ova_container_limits_cpu",
	"ova_container_limits_memory",
	"ova_container_requests_cpu",
	"ova_container_requests_memory",

	// HyperV
	"hyperv_container_limits_cpu",
	"hyperv_container_limits_memory",
	"hyperv_container_requests_cpu",
	"hyperv_container_requests_memory",

	// MCP
	"mcp_server_lightspeed_integration",
	"mcp_server_lightspeed_set_mcp_gate",

	// AAP
	"aap_url",
	"aap_token_secret_name",
	"aap_timeout",
	"aap_insecure_skip_verify",
	"aap_ca_secret_name",
}

// SupportedSettings is a map built from AllSettings filtered by SupportedSettingNames.
// It is computed at init time.
var SupportedSettings map[string]SettingDefinition

func init() {
	SupportedSettings = make(map[string]SettingDefinition, len(SupportedSettingNames))
	for _, name := range SupportedSettingNames {
		if def, ok := AllSettings[name]; ok {
			SupportedSettings[name] = def
		}
	}
}

// GetAllSettings returns AllSettings (all ForkliftController settings).
func GetAllSettings() map[string]SettingDefinition {
	return AllSettings
}

// GetAllSettingNames returns all setting names in category order.
func GetAllSettingNames() []string {
	var names []string
	for _, category := range CategoryOrder {
		var categoryNames []string
		for name, def := range AllSettings {
			if def.Category == category {
				categoryNames = append(categoryNames, name)
			}
		}
		sort.Strings(categoryNames)
		names = append(names, categoryNames...)
	}
	return names
}

// GetSettingNames returns supported setting names in category order.
func GetSettingNames() []string {
	var names []string
	for _, category := range CategoryOrder {
		var categoryNames []string
		for name, def := range SupportedSettings {
			if def.Category == category {
				categoryNames = append(categoryNames, name)
			}
		}
		sort.Strings(categoryNames)
		names = append(names, categoryNames...)
	}
	return names
}

// GetSettingsByCategory returns supported settings grouped by category.
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

// IsValidAnySetting checks if a setting name is valid in AllSettings.
func IsValidAnySetting(name string) bool {
	_, ok := AllSettings[name]
	return ok
}

// GetSettingDefinition returns the definition from SupportedSettings, or nil if not found.
func GetSettingDefinition(name string) *SettingDefinition {
	if def, ok := SupportedSettings[name]; ok {
		return &def
	}
	return nil
}

// GetAnySettingDefinition returns the definition from AllSettings, or nil if not found.
func GetAnySettingDefinition(name string) *SettingDefinition {
	if def, ok := AllSettings[name]; ok {
		return &def
	}
	return nil
}
