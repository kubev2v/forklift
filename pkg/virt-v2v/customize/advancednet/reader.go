package advancednet

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	systemHivePath = "/Windows/System32/config/SYSTEM"
	localHiveFile  = "SYSTEM.hive"
)

// networkClassGUID is the well-known GUID for the "Network Adapter" device class.
const networkClassGUID = "{4D36E972-E325-11CE-BFC1-08002BE10318}"

// ReadAdvancedNetworkSettings extracts and parses the SYSTEM hive from a
// converted disk, reading MAC addresses directly from the registry.
func ReadAdvancedNetworkSettings(diskPath, workdir string) (*AdvancedNetSettings, error) {
	hivePath := filepath.Join(workdir, localHiveFile)

	if err := extractHive(diskPath, hivePath); err != nil {
		return nil, fmt.Errorf("extract SYSTEM hive: %w", err)
	}
	defer func() { _ = os.Remove(hivePath) }()

	data, err := os.ReadFile(hivePath)
	if err != nil {
		return nil, fmt.Errorf("read SYSTEM hive: %w", err)
	}

	return ParseAdvancedNetworkSettings(data)
}

// extractHive pulls the SYSTEM hive from a raw disk image via virt-cat.
func extractHive(diskPath, outPath string) error {
	cmd := exec.Command("virt-cat",
		"--format=raw",
		"-a", diskPath,
		systemHivePath,
	)
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = out.Close()
		return fmt.Errorf("virt-cat failed: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close hive file: %w", err)
	}
	return nil
}

// ParseAdvancedNetworkSettings parses a raw SYSTEM hive and returns
// advanced network settings. MAC addresses are read directly from the
// registry (NetworkSetup2 or NetworkAddress) without needing provider input.
func ParseAdvancedNetworkSettings(hiveData []byte) (result *AdvancedNetSettings, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			retErr = fmt.Errorf("regf: corrupt hive data: %v", r)
		}
	}()

	hive, err := ParseHive(hiveData)
	if err != nil {
		return nil, err
	}

	controlSet, err := hive.ResolveCurrentControlSet()
	if err != nil {
		return nil, err
	}

	guidToMAC := buildGUIDToMAC(hive, controlSet)

	settings := &AdvancedNetSettings{}

	if err := readInterfaceSettings(hive, controlSet, guidToMAC, settings); err != nil {
		return nil, err
	}

	if err := readLanmanServer(hive, controlSet, settings); err != nil {
		return nil, err
	}

	if err := readFilePrinterSharing(hive, controlSet, guidToMAC, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// buildGUIDToMAC maps adapter GUIDs to MACs by reading them directly from
// the registry. It tries two sources in order:
//  1. NetworkSetup2\Interfaces\{GUID}\Kernel\CurrentAddress (REG_BINARY, 6 bytes)
//  2. Control\Class\{4D36E972-...}\XXXX\NetworkAddress (REG_SZ, user-set override)
func buildGUIDToMAC(hive *Hive, controlSet string) map[string]string {
	result := make(map[string]string)
	readMACsFromNetworkSetup2(hive, controlSet, result)
	readMACsFromNICClass(hive, controlSet, result)
	return result
}

// readMACsFromNetworkSetup2 populates guidToMAC from the NetworkSetup2
// Kernel\CurrentAddress or PermanentAddress binary values.
func readMACsFromNetworkSetup2(hive *Hive, controlSet string, guidToMAC map[string]string) {
	ns2Path := controlSet + "\\Control\\NetworkSetup2\\Interfaces"
	ns2Key, err := hive.OpenKey(ns2Path)
	if err != nil || ns2Key == nil {
		return
	}
	guidKeys, err := hive.EnumerateSubkeys(ns2Key)
	if err != nil {
		slog.Warn("Failed to enumerate NetworkSetup2 subkeys", "error", err)
		return
	}
	for _, gk := range guidKeys {
		guid := gk.name
		if !strings.HasPrefix(guid, "{") {
			continue
		}
		mac := readMACFromKernelKey(hive, ns2Path+"\\"+guid+"\\Kernel", guid)
		if mac != "" {
			guidToMAC[strings.ToUpper(guid)] = mac
		}
	}
}

// readMACFromKernelKey reads a 6-byte MAC from CurrentAddress or PermanentAddress.
func readMACFromKernelKey(hive *Hive, kernelPath, guid string) string {
	kernelKey, err := hive.OpenKey(kernelPath)
	if err != nil {
		slog.Warn("Failed to open Kernel subkey", "guid", guid, "error", err)
		return ""
	}
	if kernelKey == nil {
		return ""
	}
	for _, valueName := range []string{"CurrentAddress", "PermanentAddress"} {
		macBytes, found, err := hive.ReadBinary(kernelKey, valueName)
		if err != nil {
			slog.Warn("Failed to read "+valueName, "guid", guid, "error", err)
		}
		if found && len(macBytes) == 6 {
			return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
				macBytes[0], macBytes[1], macBytes[2],
				macBytes[3], macBytes[4], macBytes[5])
		}
	}
	return ""
}

// readMACsFromNICClass populates guidToMAC from the NIC device class
// NetworkAddress or OriginalNetworkAddress string values, skipping GUIDs
// already resolved by NetworkSetup2.
func readMACsFromNICClass(hive *Hive, controlSet string, guidToMAC map[string]string) {
	classPath := controlSet + "\\Control\\Class\\" + networkClassGUID
	classKey, err := hive.OpenKey(classPath)
	if err != nil || classKey == nil {
		return
	}
	subkeys, err := hive.EnumerateSubkeys(classKey)
	if err != nil {
		return
	}
	for _, sk := range subkeys {
		instanceID, found, err := hive.ReadSZ(sk, "NetCfgInstanceId")
		if err != nil || !found || instanceID == "" {
			continue
		}
		guid := strings.ToUpper(instanceID)
		if _, exists := guidToMAC[guid]; exists {
			continue
		}
		mac := readMACFromClassEntry(hive, sk, guid)
		if mac != "" {
			guidToMAC[guid] = mac
		}
	}
}

// readMACFromClassEntry reads a MAC string from NetworkAddress or OriginalNetworkAddress.
func readMACFromClassEntry(hive *Hive, sk *nkCell, guid string) string {
	for _, valueName := range []string{"NetworkAddress", "OriginalNetworkAddress"} {
		mac, found, err := hive.ReadSZ(sk, valueName)
		if err != nil {
			slog.Warn("Failed to read "+valueName, "guid", guid, "error", err)
		}
		if found && mac != "" {
			return normalizeMACAddress(mac)
		}
	}
	return ""
}

// normalizeMACAddress converts any MAC format to "AA:BB:CC:DD:EE:FF".
func normalizeMACAddress(mac string) string {
	mac = strings.ToUpper(strings.ReplaceAll(mac, "-", ""))
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, ".", "")
	if len(mac) != 12 {
		return mac
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
}

// readInterfaceSettings reads per-adapter InterfaceMetric, RegistrationEnabled,
// and NetbiosOptions from Tcpip\Parameters\Interfaces.
func readInterfaceSettings(hive *Hive, controlSet string, guidToMAC map[string]string, settings *AdvancedNetSettings) error {
	tcpipInterfaces, err := hive.OpenKey(
		controlSet + "\\Services\\Tcpip\\Parameters\\Interfaces",
	)
	if err != nil {
		return fmt.Errorf("open Tcpip\\Parameters\\Interfaces: %w", err)
	}
	if tcpipInterfaces == nil {
		return nil
	}

	guidKeys, err := hive.EnumerateSubkeys(tcpipInterfaces)
	if err != nil {
		return fmt.Errorf("enumerate interface GUIDs: %w", err)
	}

	for _, guidKey := range guidKeys {
		guid := guidKey.name
		if !strings.HasPrefix(guid, "{") || !strings.HasSuffix(guid, "}") {
			continue
		}
		mac, hasMac := guidToMAC[strings.ToUpper(guid)]
		if !hasMac || mac == "" {
			continue
		}
		iface, err := readSingleInterfaceSettings(hive, controlSet, guidKey, guid)
		if err != nil {
			return err
		}
		iface.MAC = mac
		if iface.hasNonDefaultSettings() {
			settings.Interfaces = append(settings.Interfaces, iface)
		}
	}
	return nil
}

// readSingleInterfaceSettings reads metric, DNS registration, and NetBIOS
// settings for one adapter GUID key.
func readSingleInterfaceSettings(hive *Hive, controlSet string, guidKey *nkCell, guid string) (InterfaceSettings, error) {
	iface := InterfaceSettings{}

	metric, found, err := hive.ReadDWORD(guidKey, "InterfaceMetric")
	if err != nil {
		return iface, err
	}
	if found {
		iface.InterfaceMetric = metric
	} else {
		iface.InterfaceMetricAuto = true
	}

	regEnabled, found, err := hive.ReadDWORD(guidKey, "RegistrationEnabled")
	if err != nil {
		return iface, err
	}
	if found {
		iface.RegistrationEnabled = regEnabled
	} else {
		iface.RegistrationEnabled = DNSRegistrationEnabled
	}

	netbtPath := controlSet + "\\Services\\NetBT\\Parameters\\Interfaces\\Tcpip_" + guid
	netbtKey, err := hive.OpenKey(netbtPath)
	if err != nil {
		return iface, err
	}
	if netbtKey != nil {
		nbOpt, found, err := hive.ReadDWORD(netbtKey, "NetbiosOptions")
		if err != nil {
			return iface, err
		}
		if found {
			iface.NetbiosOptions = nbOpt
		}
	}

	return iface, nil
}

// hasNonDefaultSettings returns true if at least one setting deviates from
// the Windows default.
func (i InterfaceSettings) hasNonDefaultSettings() bool {
	if !i.InterfaceMetricAuto && i.InterfaceMetric != 0 {
		return true
	}
	if i.RegistrationEnabled != DNSRegistrationEnabled {
		return true
	}
	return i.NetbiosOptions == NetbiosOptionsEnabled || i.NetbiosOptions == NetbiosOptionsDisabled
}

// readLanmanServer reads the LanmanServer service start type.
func readLanmanServer(hive *Hive, controlSet string, settings *AdvancedNetSettings) error {
	lanmanKey, err := hive.OpenKey(controlSet + "\\Services\\LanmanServer")
	if err != nil {
		return err
	}
	if lanmanKey == nil {
		return nil
	}
	start, found, err := hive.ReadDWORD(lanmanKey, "Start")
	if err != nil {
		return err
	}
	if found {
		settings.LanmanServerStart = start
	}
	return nil
}

// readFilePrinterSharing determines which adapters have File & Printer Sharing
// disabled by comparing LanmanServer\Linkage\Bind against all interface GUIDs.
func readFilePrinterSharing(hive *Hive, controlSet string, guidToMAC map[string]string, settings *AdvancedNetSettings) error {
	boundGUIDs, err := readBoundGUIDs(hive, controlSet)
	if err != nil {
		return err
	}
	if boundGUIDs == nil {
		return nil
	}

	tcpipInterfaces, err := hive.OpenKey(
		controlSet + "\\Services\\Tcpip\\Parameters\\Interfaces",
	)
	if err != nil || tcpipInterfaces == nil {
		return err
	}
	guidKeys, err := hive.EnumerateSubkeys(tcpipInterfaces)
	if err != nil {
		return err
	}
	for _, gk := range guidKeys {
		guid := gk.name
		if !strings.HasPrefix(guid, "{") {
			continue
		}
		upperGUID := strings.ToUpper(guid)
		if boundGUIDs[upperGUID] {
			continue
		}
		mac, hasMac := guidToMAC[upperGUID]
		if !hasMac || mac == "" {
			continue
		}
		settings.FilePrinterSharingDisabled = append(
			settings.FilePrinterSharingDisabled,
			AdapterRef{GUID: guid, MAC: mac},
		)
	}
	return nil
}

// readBoundGUIDs parses the LanmanServer\Linkage\Bind multi-string value and
// returns the set of adapter GUIDs that have File & Printer Sharing bound.
// Returns nil if the Linkage key or Bind value is absent.
func readBoundGUIDs(hive *Hive, controlSet string) (map[string]bool, error) {
	linkageKey, err := hive.OpenKey(controlSet + "\\Services\\LanmanServer\\Linkage")
	if err != nil {
		return nil, err
	}
	if linkageKey == nil {
		return nil, nil
	}
	bindValues, found, err := hive.ReadMultiSZ(linkageKey, "Bind")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	result := make(map[string]bool)
	for _, bind := range bindValues {
		for _, p := range strings.Split(bind, "_") {
			if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
				result[strings.ToUpper(p)] = true
			}
		}
	}
	return result, nil
}

// WriteSettingsFile writes the settings as JSON to workdir.
func WriteSettingsFile(settings *AdvancedNetSettings, workdir string) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal advanced net settings: %w", err)
	}
	outPath := filepath.Join(workdir, AdvancedNetSettingsFile)
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("write advanced net settings: %w", err)
	}
	slog.Info("Advanced network settings written", "path", outPath)
	return nil
}

// ReadSettingsFile loads the settings JSON from workdir; returns nil if absent.
func ReadSettingsFile(workdir string) (*AdvancedNetSettings, error) {
	path := filepath.Join(workdir, AdvancedNetSettingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // nil signals "no file found" to caller
		}
		return nil, err
	}
	var settings AdvancedNetSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("unmarshal advanced net settings: %w", err)
	}
	return &settings, nil
}
