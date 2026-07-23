package netcfg

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// NM parses NetworkManager keyfile-format connection profiles.
// Multiple concatenated profiles are split on [connection] boundaries.
// Addresses under [ipv4] go to IPv4, addresses under [ipv6] go to IPv6.
func NM(section string) ([]api.InterfaceInfo, error) {
	var interfaces []api.InterfaceInfo
	var current api.InterfaceInfo
	current.Source = "nm-connection"
	currentSection := ""

	scanner := bufio.NewScanner(strings.NewReader(section))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[connection]" {
			if current.Name != "" || current.HasIPs() {
				interfaces = append(interfaces, current)
				current = api.InterfaceInfo{Source: "nm-connection"}
			}
			currentSection = "connection"
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "interface-name":
			current.Name = val
		case "mac-address":
			current.MAC = strings.ToLower(val)
		case "method":
			if (currentSection == "ipv4" || currentSection == "ipv6") && val == "auto" {
				current.DHCP = true
			}
		}
		if (currentSection == "ipv4" || currentSection == "ipv6") && strings.HasPrefix(key, "address") && strings.Contains(val, "/") {
			ip := strings.SplitN(val, "/", 2)[0]
			if ip != "" {
				switch currentSection {
				case "ipv6":
					current.IPv6 = append(current.IPv6, ip)
				default:
					current.IPv4 = append(current.IPv4, ip)
				}
			}
		}
	}
	if current.Name != "" || current.HasIPs() {
		interfaces = append(interfaces, current)
	}
	if err := scanner.Err(); err != nil {
		return interfaces, fmt.Errorf("NM parser: scanner error: %w", err)
	}
	return interfaces, nil
}
