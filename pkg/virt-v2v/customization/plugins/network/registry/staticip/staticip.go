// Package staticip parses the StaticIPs format string and builds per-NIC
// IP configurations used by the registry network plugin templates.
package staticip

import (
	"fmt"
	"sort"
	"strings"
)

// IPEntry holds a single IP address configuration for one NIC.
type IPEntry struct {
	IP           string
	Gateway      string
	PrefixLength string
	DNS          []string
}

// IPConfig groups all complementary (non-primary) IPs for one MAC address.
type IPConfig struct {
	MAC string
	IPs []IPEntry
}

// ParseEntries splits the underscore-delimited static IP string into
// per-segment entries grouped by normalized MAC address.
// Segments that don't match the expected "MAC:ip:IP,GW,PREFIX,DNS..." format
// are collected as warnings (non-fatal) and the segment is skipped.
func ParseEntries(staticIPs string) (map[string][]IPEntry, []error) {
	macMap := map[string][]IPEntry{}
	var warnings []error
	for _, segment := range strings.Split(staticIPs, "_") {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		parts := strings.SplitN(segment, ":ip:", 2)
		if len(parts) != 2 {
			warnings = append(warnings, fmt.Errorf("ParseEntries: malformed segment %q: missing ':ip:' delimiter", segment))
			continue
		}
		mac := strings.ReplaceAll(parts[0], ":", "-")
		ipParts := strings.Split(parts[1], ",")
		if len(ipParts) < 5 {
			warnings = append(warnings, fmt.Errorf("ParseEntries: malformed segment %q: expected at least 5 comma-separated fields after ':ip:', got %d", segment, len(ipParts)))
			continue
		}
		var dns []string
		for _, d := range ipParts[3:] {
			d = strings.TrimSpace(d)
			if d != "" {
				dns = append(dns, d)
			}
		}
		macMap[mac] = append(macMap[mac], IPEntry{
			IP:           ipParts[0],
			Gateway:      ipParts[1],
			PrefixLength: ipParts[2],
			DNS:          dns,
		})
	}
	return macMap, warnings
}

// BuildComplementaryConfigs takes parsed MAC→entries and returns only
// MACs that have more than one IP entry, dropping the primary (first) IP.
// Results are sorted by MAC for deterministic output.
func BuildComplementaryConfigs(macMap map[string][]IPEntry) []IPConfig {
	var macs []string
	for mac := range macMap {
		macs = append(macs, mac)
	}
	sort.Strings(macs)

	var configs []IPConfig
	for _, mac := range macs {
		ips := macMap[mac]
		if len(ips) > 1 {
			configs = append(configs, IPConfig{MAC: mac, IPs: ips[1:]})
		}
	}
	return configs
}
