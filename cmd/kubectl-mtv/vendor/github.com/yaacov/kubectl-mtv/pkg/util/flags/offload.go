package flags

import (
	"fmt"
	"strings"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// OffloadVendors returns the list of supported storage copy-offload vendor products
// directly from the Forklift API types (single source of truth).
func offloadVendorStrings() []string {
	products := v1beta1.StorageVendorProducts()
	vendors := make([]string, len(products))
	for i, p := range products {
		vendors[i] = string(p)
	}
	return vendors
}

// OffloadVendors is the list of supported offload vendor identifiers,
// derived from kubev2v/forklift StorageVendorProducts().
var OffloadVendors = offloadVendorStrings()

// OffloadVendorHelp is the help text for --default-offload-vendor flags.
var OffloadVendorHelp = "Default offload plugin vendor for storage pairs (" + strings.Join(OffloadVendors, "|") + ")"

// ValidateOffloadVendor returns nil if the vendor is in the supported list, or an error otherwise.
func ValidateOffloadVendor(vendor string) error {
	for _, v := range OffloadVendors {
		if vendor == v {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(OffloadVendors, "|"))
}

// OffloadPlugins is the list of supported offload plugin types.
var OffloadPlugins = []string{"vsphere", "vsphereNfs", "csiVolumeImport"}

// OffloadPluginHelp is the help text for --default-offload-plugin flags.
var OffloadPluginHelp = "Default offload plugin type for storage pairs (" + strings.Join(OffloadPlugins, "|") + ")"

// StoragePairOffloadNote documents plugin-specific offload constraints for storage-pair flags.
const StoragePairOffloadNote = "With offloadPlugin=csiVolumeImport, offloadVendor is primera3par|ontap only. With offloadPlugin=vsphereNfs, offloadVendor is pureFlashArray only, volumeMode must be Filesystem, and offloadMigrationHosts is not allowed"

// NfsOffloadPlugin is the CLI id for vSphere NFS array copy offload.
const NfsOffloadPlugin = "vsphereNfs"

// NfsOffloadVendor is the only StorageVendorProduct accepted by vsphereNfs.
const NfsOffloadVendor = "pureFlashArray"

// ValidateOffloadPlugin returns nil if the plugin is in the supported list, or an error otherwise.
func ValidateOffloadPlugin(plugin string) error {
	for _, p := range OffloadPlugins {
		if plugin == p {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(OffloadPlugins, "|"))
}

// BuildOffloadPlugin builds an OffloadPlugin from CLI storage-pair values.
// volumeMode may be empty; when set, vsphereNfs requires Filesystem.
func BuildOffloadPlugin(plugin, vendor, secret, migrationHosts, volumeMode string) (*v1beta1.OffloadPlugin, error) {
	if err := ValidateOffloadPlugin(plugin); err != nil {
		return nil, fmt.Errorf("invalid offloadPlugin '%s': %w", plugin, err)
	}
	if err := ValidateOffloadVendor(vendor); err != nil {
		return nil, fmt.Errorf("invalid offloadVendor '%s': %w", vendor, err)
	}

	switch plugin {
	case "vsphere":
		xcopyConfig := &v1beta1.VSphereXcopyPluginConfig{
			SecretRef:            secret,
			StorageVendorProduct: v1beta1.StorageVendorProduct(vendor),
		}
		if migrationHosts != "" {
			for _, h := range strings.Split(migrationHosts, "+") {
				h = strings.TrimSpace(h)
				if h != "" {
					xcopyConfig.DedicatedMigrationHosts = append(xcopyConfig.DedicatedMigrationHosts, h)
				}
			}
		}
		return &v1beta1.OffloadPlugin{VSphereXcopyPluginConfig: xcopyConfig}, nil
	case NfsOffloadPlugin:
		if err := validateNfsOffload(vendor, migrationHosts, volumeMode); err != nil {
			return nil, err
		}
		return &v1beta1.OffloadPlugin{
			VSphereNfsCopyPluginConfig: &v1beta1.VSphereNfsCopyPluginConfig{
				SecretRef:            secret,
				StorageVendorProduct: v1beta1.StorageVendorProduct(vendor),
			},
		}, nil
	case "csiVolumeImport":
		return &v1beta1.OffloadPlugin{
			CsiVolumeImport: &v1beta1.CsiVolumeImport{
				SecretRef:            secret,
				StorageVendorProduct: v1beta1.StorageVendorProduct(vendor),
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown offload plugin '%s': supported plugins are: %s", plugin, strings.Join(OffloadPlugins, ", "))
	}
}

func validateNfsOffload(vendor, migrationHosts, volumeMode string) error {
	if vendor != NfsOffloadVendor {
		return fmt.Errorf("offloadPlugin %s only supports offloadVendor %s", NfsOffloadPlugin, NfsOffloadVendor)
	}
	if strings.TrimSpace(migrationHosts) != "" {
		return fmt.Errorf("offloadMigrationHosts is not supported with offloadPlugin %s", NfsOffloadPlugin)
	}
	if volumeMode != "" && volumeMode != "Filesystem" {
		return fmt.Errorf("offloadPlugin %s requires volumeMode Filesystem", NfsOffloadPlugin)
	}
	return nil
}
