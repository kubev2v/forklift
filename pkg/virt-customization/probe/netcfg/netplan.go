package netcfg

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"gopkg.in/yaml.v2"
)

type netplanConfig struct {
	Network struct {
		Renderer  string                   `yaml:"renderer"`
		Ethernets map[string]netplanDevice `yaml:"ethernets"`
		Bonds     map[string]netplanDevice `yaml:"bonds"`
		Bridges   map[string]netplanDevice `yaml:"bridges"`
		Vlans     map[string]netplanDevice `yaml:"vlans"`
	} `yaml:"network"`
}

// NetplanResult holds both parsed interfaces and metadata.
type NetplanResult struct {
	Interfaces []api.InterfaceInfo
	Renderer   string // "networkd", "NetworkManager", or empty
}

type netplanDevice struct {
	Addresses []string     `yaml:"addresses"`
	DHCP4     bool         `yaml:"dhcp4"`
	DHCP6     bool         `yaml:"dhcp6"`
	Match     netplanMatch `yaml:"match"`
}

type netplanMatch struct {
	MACAddress string `yaml:"macaddress"`
}

// Netplan parses netplan YAML configuration.
// The input may contain multiple concatenated YAML documents (one per file).
func Netplan(section string) (NetplanResult, error) {
	var result NetplanResult
	var errs []error

	for i, doc := range splitYAMLDocuments(section) {
		var cfg netplanConfig
		if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
			errs = append(errs, fmt.Errorf("parsing netplan document %d: %w", i, err))
			continue
		}
		if cfg.Network.Renderer != "" && result.Renderer == "" {
			result.Renderer = cfg.Network.Renderer
		}
		result.Interfaces = append(result.Interfaces, extractDevices(cfg.Network.Ethernets, "netplan")...)
		result.Interfaces = append(result.Interfaces, extractDevices(cfg.Network.Bonds, "netplan")...)
		result.Interfaces = append(result.Interfaces, extractDevices(cfg.Network.Bridges, "netplan")...)
		result.Interfaces = append(result.Interfaces, extractDevices(cfg.Network.Vlans, "netplan")...)
	}
	return result, errors.Join(errs...)
}

// extractDevices converts a netplan device map into sorted InterfaceInfo entries.
func extractDevices(devices map[string]netplanDevice, source string) []api.InterfaceInfo {
	names := make([]string, 0, len(devices))
	for name := range devices {
		names = append(names, name)
	}
	sort.Strings(names)

	var result []api.InterfaceInfo
	for _, name := range names {
		dev := devices[name]
		iface := api.InterfaceInfo{
			Name:   name,
			MAC:    strings.ToLower(dev.Match.MACAddress),
			DHCP:   dev.DHCP4 || dev.DHCP6,
			Source: source,
		}
		for _, addr := range dev.Addresses {
			ip := strings.SplitN(addr, "/", 2)[0]
			if ip == "" {
				continue
			}
			if strings.Contains(ip, ":") {
				iface.IPv6 = append(iface.IPv6, ip)
			} else {
				iface.IPv4 = append(iface.IPv4, ip)
			}
		}
		result = append(result, iface)
	}
	return result
}

// splitYAMLDocuments splits concatenated YAML docs on "---" separators.
// Also treats the entire input as a single doc if no separators exist.
func splitYAMLDocuments(input string) []string {
	var docs []string
	var current strings.Builder

	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "---" {
			if current.Len() > 0 {
				docs = append(docs, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		docs = append(docs, current.String())
	}
	return docs
}
