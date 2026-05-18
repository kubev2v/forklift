package advancednet

// AdvancedNetSettingsFile is the filename written to Workdir.
const AdvancedNetSettingsFile = "advanced_net.json"

// Windows default values for network settings.
const (
	LanmanServerStartAutomatic uint32 = 2 // Service starts at boot (default)
	DNSRegistrationEnabled     uint32 = 1 // Adapter registers in DNS (default)
	NetbiosOptionsDefault      uint32 = 0 // Use DHCP-assigned setting (default)
	NetbiosOptionsEnabled      uint32 = 1
	NetbiosOptionsDisabled     uint32 = 2
)

// AdvancedNetSettings holds per-interface and global network settings
// extracted from a Windows SYSTEM registry hive. Only non-default values
// are populated, the consumer should skip writing any zero-valued field.
type AdvancedNetSettings struct {
	Interfaces                 []InterfaceSettings `json:"interfaces"`
	LanmanServerStart          uint32              `json:"lanmanServerStart"`
	FilePrinterSharingDisabled []AdapterRef        `json:"filePrinterSharingDisabledAdapters"`
}

// AdapterRef identifies a network adapter by both GUID and MAC so the
// firstboot script can match by MAC when the GUID changes after migration.
type AdapterRef struct {
	GUID string `json:"guid"`
	MAC  string `json:"mac,omitempty"`
}

// InterfaceSettings captures per-NIC advanced settings. MAC is the adapter's
// hardware address (resolved from the SYSTEM hive when possible, otherwise
// the GUID is stored as a fallback identifier).
type InterfaceSettings struct {
	MAC                 string `json:"mac"`
	InterfaceMetric     uint32 `json:"interfaceMetric"`
	InterfaceMetricAuto bool   `json:"interfaceMetricAuto"`
	RegistrationEnabled uint32 `json:"registrationEnabled"`
	NetbiosOptions      uint32 `json:"netbiosOptions"`
}

// HasNonDefaultSettings returns true when at least one setting differs from
// the Windows default, meaning a firstboot script should be generated.
func (s *AdvancedNetSettings) HasNonDefaultSettings() bool {
	if s.LanmanServerStart != 0 && s.LanmanServerStart != LanmanServerStartAutomatic {
		return true
	}
	if len(s.FilePrinterSharingDisabled) > 0 {
		return true
	}
	for _, iface := range s.Interfaces {
		if !iface.InterfaceMetricAuto && iface.InterfaceMetric != 0 {
			return true
		}
		if iface.RegistrationEnabled != DNSRegistrationEnabled {
			return true
		}
		if iface.NetbiosOptions == NetbiosOptionsEnabled || iface.NetbiosOptions == NetbiosOptionsDisabled {
			return true
		}
	}
	return false
}
